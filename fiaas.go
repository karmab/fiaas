package main

import (
	//	"encoding/json"
	"fmt"
	"github.com/rackspace/gophercloud"
	"github.com/rackspace/gophercloud/openstack"
	"log"
	"net/http"
	"strings"
)

var tenant string
var subnet string

func authenticate(username string, password string, tenant string) bool {
	fmt.Println("username:", username)
	fmt.Println("password:", password)
	opts := gophercloud.AuthOptions{
		IdentityEndpoint: "http://192.168.122.162:5000/v2.0",
		Username:         username,
		Password:         password,
		TenantName:       tenant,
	}
	_, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		fmt.Println("failed to authenticate")
		return false
	}
	return true
}

func getip(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	for k, v := range r.Form {
		if k == "tenant" {
			tenant = strings.Join(v, "")
			fmt.Println("tenant:", tenant)
		}
		if k == "subnet" {
			subnet = strings.Join(v, "")
			fmt.Println("subnet:", subnet)
		}
	}
	if tenant == "" || subnet == "" {
		fmt.Fprintf(w, "Missing Data")
	}
	username, password, _ := r.BasicAuth()
	if username == "" || password == "" {
		fmt.Fprintf(w, "Missing Credentials")
	}
	authorized := authenticate(username, password, tenant)
	if authorized == false {
		fmt.Fprintf(w, "Wrong Credentials")
	}
	fmt.Fprintf(w, "Ready to process") // write data to response
}

func main() {
	http.HandleFunc("/getip", getip)
	err := http.ListenAndServe(":7000", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
