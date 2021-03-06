package main

import (
	"encoding/json"
	"github.com/go-ini/ini"
	"github.com/rackspace/gophercloud"
	"github.com/rackspace/gophercloud/openstack"
	"github.com/rackspace/gophercloud/openstack/identity/v2/tenants"
	"github.com/rackspace/gophercloud/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/rackspace/gophercloud/openstack/networking/v2/ports"
	"github.com/rackspace/gophercloud/openstack/networking/v2/subnets"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
)

var config *Config
var mappings map[string][]string
var blacklist []string

type Keystone struct {
	Endpoint      string `ini:"auth_url"`
	AdminUser     string `ini:"admin_user"`
	AdminPassword string `ini:"admin_password"`
	AdminTenant   string `ini:"admin_tenant"`
}

type Defaults struct {
	Host    string `ini:"host"`
	Port    string `ini:"port"`
	Ssl     bool   `ini:"ssl"`
	Cert    string `ini:"cert"`
	Key     string `ini:"key"`
	LogDir  string `ini:"log_dir"`
	LogFile string `ini:"log_file"`
	Debug   bool   `ini:"debug"`
}

type Rbac struct {
	Enabled   bool   `ini:"enabled"`
	Mappings  string `ini:"mappings"`
	Blacklist string `ini:"blacklist"`
}

type Config struct {
	Defaults Defaults `ini:"DEFAULT"`
	Keystone Keystone `ini:"keystone"`
	Rbac     Rbac     `ini:"rbac"`
}

type Error struct {
	Message string `json:"message"`
}

type Data struct {
	Subnet string `json:"subnet"`
	Tenant string `json:"tenant"`
	Ip     string `json:"ip"`
}

func ConvertMappings(mappings string) map[string][]string {
	var results = make(map[string][]string)
	for _, entry := range strings.Split(mappings, ",") {
		info := strings.Split(entry, ":")
		results[info[0]] = make([]string, 0)
		for _, subnet := range strings.Split(info[1], "+") {
			results[info[0]] = append(results[info[0]], subnet)
		}
	}
	return results
}

func ConvertBlacklist(blacklist string) []string {
	var results []string
	for _, entry := range strings.Split(blacklist, ",") {
		results = append(results, entry)
	}
	return results
}

func Ips(cidr string) ([]string, error) {
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return nil, err
	}
	var ips []string
	for ip := ip.Mask(ipnet.Mask); ipnet.Contains(ip); inc(ip) {
		ips = append(ips, ip.String())
	}
	// remove network address and broadcast address
	return ips[2 : len(ips)-1], nil
}

//  http://play.golang.org/p/m8TNTtygK0
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

//func GetPorts(n *gophercloud.ServiceClient, cidr string) []ports.Port {
func GetPorts(n *gophercloud.ServiceClient, cidr string) map[string]bool {
	_, networkrange, _ := net.ParseCIDR(cidr)
	results := make(map[string]bool)
	allpages, _ := ports.List(n, nil).AllPages()
	portList, _ := ports.ExtractPorts(allpages)
	for _, p := range portList {
		if len(p.FixedIPs) > 0 {
			for _, entry := range p.FixedIPs {
				ip := net.ParseIP(entry.IPAddress)
				if networkrange.Contains(ip) {
					results[entry.IPAddress] = true
				}
			}

		}
	}
	return results
}

func GetSubnets(n *gophercloud.ServiceClient) map[string]subnets.Subnet {
	results := make(map[string]subnets.Subnet)
	allpages, _ := subnets.List(n, nil).AllPages()
	subnetList, _ := subnets.ExtractSubnets(allpages)
	for _, s := range subnetList {
		results[s.Name] = s
	}
	return results
}

func GetTenantID(identity *gophercloud.ServiceClient, tenant string) string {
	var tenantid string
	allpages, _ := tenants.List(identity, nil).AllPages()
	tenantList, _ := tenants.ExtractTenants(allpages)
	for _, t := range tenantList {
		if t.Name == tenant {
			tenantid = t.ID
		}
	}
	return tenantid
}

var tenant string
var subnet string

func authenticate(endpoint string, username string, password string, tenant string) string {
	opts := gophercloud.AuthOptions{
		IdentityEndpoint: endpoint,
		Username:         username,
		Password:         password,
		TenantName:       tenant,
	}
	auth, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		return ""
	}
	k := openstack.NewIdentityV2(auth)
	return GetTenantID(k, tenant)
}

func getip(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	r.ParseForm()
	for k, v := range r.Form {
		if k == "tenant" {
			tenant = strings.Join(v, "")
		}
		if k == "subnet" {
			subnet = strings.Join(v, "")
		}
	}
	if tenant == "" || subnet == "" {
		w.WriteHeader(404)
		message := Error{Message: "Missing Data"}
		response, _ := json.Marshal(message)
		w.Write(response)
		return
	}
	username, password, _ := r.BasicAuth()
	if username == "" || password == "" {
		w.WriteHeader(401)
		message := Error{Message: "Missing Credentials"}
		response, _ := json.Marshal(message)
		w.Write(response)
		return
	}
	tenantid := authenticate(config.Keystone.Endpoint, username, password, tenant)
	if tenantid == "" {
		w.WriteHeader(401)
		message := Error{Message: "Wrong Credentials"}
		response, _ := json.Marshal(message)
		w.Write(response)
		return
	}
	admincredentials := gophercloud.AuthOptions{
		IdentityEndpoint: config.Keystone.Endpoint,
		Username:         config.Keystone.AdminUser,
		Password:         config.Keystone.AdminPassword,
		TenantName:       config.Keystone.AdminTenant,
	}
	auth, _ := openstack.AuthenticatedClient(admincredentials)
	n, _ := openstack.NewNetworkV2(auth, gophercloud.EndpointOpts{
		Name:   "neutron",
		Region: "RegionOne",
	})
	subnets := GetSubnets(n)
	subnetinfo, ok := subnets[subnet]
	if ok == false {
		w.WriteHeader(404)
		message := Error{Message: "Subnet not found"}
		response, _ := json.Marshal(message)
		w.Write(response)
		return
	}
	if config.Rbac.Enabled == true {
		if len(mappings) != 0 {
			mappingsinfo, ok := mappings[tenant]
			if ok == false {
				w.WriteHeader(401)
				message := Error{Message: "Subnet not allowed by RBAC"}
				response, _ := json.Marshal(message)
				w.Write(response)
				return
			}
			subnetfound := false
			for _, s := range mappingsinfo {
				if s == subnet {
					subnetfound = true
				}
			}
			if !subnetfound {
				w.WriteHeader(401)
				message := Error{Message: "Subnet not allowed by RBAC"}
				response, _ := json.Marshal(message)
				w.Write(response)
				return
			}
		}
	}
	networkid := subnetinfo.NetworkID
	cidr := subnetinfo.CIDR
	floatings := GetPorts(n, cidr)
	if len(blacklist) > 0 {
		for _, element := range blacklist {
			floatings[element] = true
		}
	}
	ips, _ := Ips(cidr)
	var ip string
	for _, i := range ips {
		if _, ok := floatings[i]; !ok {
			ip = i
			break
		}
	}
	opts := floatingips.CreateOpts{
		FloatingNetworkID: networkid,
		FloatingIP:        ip,
		TenantID:          tenantid,
	}
	if config.Defaults.Debug {
		log.Printf("Floatingip data:%s\n", opts)
	}
	_, err := floatingips.Create(n, opts).Extract()
	if err != nil {
		log.Printf("Couldnt associate floating because of %v", err)
		w.WriteHeader(403)
		message := Error{Message: err.Error()}
		response, _ := json.Marshal(message)
		w.Write(response)

	} else {
		log.Printf("Associating ip %s from subnet %s to tenant %s\n", ip, subnet, tenant)
		w.WriteHeader(200)
		data := Data{Subnet: subnet, Tenant: tenant, Ip: ip}
		response, _ := json.Marshal(data)
		w.Write(response)
	}
	return
}

func main() {
	defaults := Defaults{Host: "0.0.0.0", Port: "7000", Ssl: false, Debug: false, Cert: "/etc/fiaas.crt", Key: "/etc/fiaas.key", LogFile: "fiaas.log", LogDir: "/var/log"}
	keystone := Keystone{AdminUser: "admin", AdminTenant: "admin"}
	rbac := Rbac{Enabled: false}
	config = &Config{Defaults: defaults, Keystone: keystone, Rbac: rbac}
	ini.MapTo(config, "/etc/fiaas.conf")
	mappings = ConvertMappings(config.Rbac.Mappings)
	blacklist = ConvertBlacklist(config.Rbac.Blacklist)
	out, _ := os.Create(config.Defaults.LogDir + "/" + config.Defaults.LogFile)
	//out, _ := os.OpenFile(config.Defaults.LogDir+"/"+config.Defaults.LogFile, os.O_APPEND, 0666)
	log.SetOutput(out)
	http.HandleFunc("/getip", getip)
	if config.Defaults.Ssl == true {
		err := http.ListenAndServeTLS(config.Defaults.Host+":"+config.Defaults.Port, config.Defaults.Cert, config.Defaults.Key, nil)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	} else {
		err := http.ListenAndServe(config.Defaults.Host+":"+config.Defaults.Port, nil)
		if err != nil {
			log.Fatal("ListenAndServe: ", err)
		}
	}
}
