package main

import (
	//	"encoding/json"
	"fmt"
	"github.com/rackspace/gophercloud"
	"github.com/rackspace/gophercloud/openstack"
	"github.com/rackspace/gophercloud/openstack/identity/v2/tenants"
	"github.com/rackspace/gophercloud/openstack/networking/v2/extensions/layer3/floatingips"
	"github.com/rackspace/gophercloud/openstack/networking/v2/ports"
	"github.com/rackspace/gophercloud/openstack/networking/v2/subnets"
	"log"
	"net"
	"net/http"
	"strings"
)

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

type Config struct {
	Blacklist []string
	Endpoint  string
	Username  string
	Password  string
	Tenant    string
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
					//results = append(results, entry.IPAddress)
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
		fmt.Println("failed to authenticate")
		return ""
	}
	k := openstack.NewIdentityV2(auth)
	return GetTenantID(k, tenant)
}

func getip(w http.ResponseWriter, r *http.Request) {
	config := Config{Endpoint: "http://192.168.122.162:5000/v2.0", Username: "admin", Password: "unix1234", Tenant: "admin"}
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
		fmt.Fprintf(w, "Missing Data")
	}
	username, password, _ := r.BasicAuth()
	if username == "" || password == "" {
		fmt.Fprintf(w, "Missing Credentials")
	}
	tenantid := authenticate(config.Endpoint, username, password, tenant)
	if tenantid == "" {
		fmt.Fprintf(w, "Wrong Credentials")
		return
	}
	admincredentials := gophercloud.AuthOptions{
		IdentityEndpoint: config.Endpoint,
		Username:         config.Username,
		Password:         config.Password,
		TenantName:       config.Tenant,
	}
	auth, _ := openstack.AuthenticatedClient(admincredentials)
	n, _ := openstack.NewNetworkV2(auth, gophercloud.EndpointOpts{
		Name:   "neutron",
		Region: "RegionOne",
	})
	subnets := GetSubnets(n)
	subnetinfo, ok := subnets[subnet]
	if ok == false {
		fmt.Fprintf(w, "Subnet %s not found", subnet)
		return
	}
	networkid := subnetinfo.NetworkID
	cidr := subnetinfo.CIDR
	floatings := GetPorts(n, cidr)
	ips, _ := Ips(cidr)
	var ip string
	for _, i := range ips {
		if _, ok := floatings[i]; !ok {
			ip = i
			break
		}
	}
	fmt.Println(ip)
	opts := floatingips.CreateOpts{
		FloatingNetworkID: networkid,
		FloatingIP:        ip,
		TenantID:          tenantid,
	}
	floatingip, _ := floatingips.Create(n, opts).Extract()
	fmt.Println(floatingip)
}

func main() {
	http.HandleFunc("/getip", getip)
	err := http.ListenAndServe(":7000", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
