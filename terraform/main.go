package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/plugin"
	"github.com/hashicorp/terraform/terraform"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type Server struct {
	Endpoint string
	User     string
	Password string
}

type Credentials struct {
	Tenant string
	Subnet string
}

type Net struct {
	Tenant string `json:"tenant"`
	Subnet string `json:"subent"`
	Ip     string `json:"ip"`
}

type Error struct {
	Message string `json:"message"`
}

type Data struct {
	Net   Net   `json:"data"`
	Error Error `json:"error"`
}

func (c *Server) GetIp(credentials *Credentials) Data {
	data1 := url.Values{}
	data1.Add("tenant", credentials.Tenant)
	data1.Add("subnet", credentials.Subnet)
	transCfg := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // ignore expired SSL certificates
	}
	req, err := http.NewRequest("POST", c.Endpoint, strings.NewReader(data1.Encode()))
	req.SetBasicAuth(c.User, c.Password)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	//cli := &http.Client{}
	cli := &http.Client{Transport: transCfg}
	resp, err := cli.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	data := Data{}
	err = decoder.Decode(&data)
	if err != nil {
		log.Fatalln(err)
	}
	return data
}

func main() {
	opts := plugin.ServeOpts{
		ProviderFunc: Provider,
	}
	plugin.Serve(&opts)
}

func Provider() terraform.ResourceProvider {
	return &schema.Provider{
		Schema:        providerSchema(),
		ResourcesMap:  providerResources(),
		ConfigureFunc: providerConfigure,
	}
}

// List of supported configuration fields for your provider.
// Here we define a linked list of all the fields that we want to
// support in our provider (api_key, endpoint, timeout & max_retries).
// More info in https://github.com/hashicorp/terraform/blob/v0.6.6/helper/schema/schema.go#L29-L142
func providerSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"endpoint": &schema.Schema{
			Type:        schema.TypeString,
			Required:    true,
			Description: "The URL to the API",
		},
		"user": &schema.Schema{
			Type:        schema.TypeString,
			Required:    true,
			Description: "User",
		},
		"password": &schema.Schema{
			Type:        schema.TypeString,
			Required:    true,
			Description: "Password",
		},
	}
}

// List of supported resources and their configuration fields.
// Here we define da linked list of all the resources that we want to
// support in our provider. As an example, if you were to write an AWS provider
// which supported resources like ec2 instances, elastic balancers and things of that sort
// then this would be the place to declare them.
// More info here https://github.com/hashicorp/terraform/blob/v0.6.6/helper/schema/resource.go#L17-L81
func providerResources() map[string]*schema.Resource {
	return map[string]*schema.Resource{
		"fiaas_ip": &schema.Resource{
			SchemaVersion: 1,
			Create:        createFunc,
			Read:          readFunc,
			Update:        updateFunc,
			Delete:        deleteFunc,
			Schema: map[string]*schema.Schema{ // List of supported configuration fields for your resource
				"tenant": &schema.Schema{
					Type:     schema.TypeString,
					Required: true,
				},
				"subnet": &schema.Schema{
					Type:     schema.TypeString,
					Required: true,
				},
			},
		},
	}
}

// This is the function used to fetch the configuration params given
// to our provider which we will use to initialise a dummy client that
// interacts with the API.
func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	client := Server{
		Endpoint: d.Get("endpoint").(string),
		User:     d.Get("user").(string),
		Password: d.Get("password").(string),
	}

	return &client, nil
}

func createFunc(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*Server)
	credentials := Credentials{
		Tenant: d.Get("tenant").(string),
		Subnet: d.Get("subnet").(string),
	}

	result := client.GetIp(&credentials)
	if result.Net.Ip == "" {
		return errors.New(result.Error.Message)
	}
	d.SetId(result.Net.Ip)
	return nil
}

func readFunc(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func updateFunc(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func deleteFunc(d *schema.ResourceData, meta interface{}) error {
	return nil
}
