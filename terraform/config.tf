provider "fiaas" {
  endpoint    = "https://192.168.122.162:7000/getip"
  user        = "testk"
  password    = "testk"
  insecure    = true
}

resource "fiaas_ip" "dani" {
  tenant = "testk"
  subnet = "dev"
}
