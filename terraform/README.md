# terraform fiaas plugin

## Installation

to compile you need to grab 0.8.6 version of terraform

```
go get github.com/hashicorp/terraform
cd $GOPATH/src/github.com/hashicorp/terraform
git checkout tags/v0.8.6
```

you can then run  the following to compile

```
make build
```

## Usage

```
provider "fiaas" {
  endpoint    = "http://192.168.122.162:7000/getip"
  user        = "testk"
  password    = "testk"
}

resource "fiaas_ip" "my-speedy-ip5" {
  tenant = "testk"
  subnet = "pro"
}
```


