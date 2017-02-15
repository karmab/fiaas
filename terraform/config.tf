provider "fiaas" {
  endpoint    = "http://192.168.122.162:7000/getip"
  user        = "testk"
  password    = "testk"
}

resource "fiaas_ip" "my-speedy-ip5" {
  tenant = "testk"
  subnet = "pro"
}
