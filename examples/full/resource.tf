terraform {
  required_providers {
    spheron = {
      version = "0.1"
      source  = "spheron/spheron"
    }
  }
}

provider "spheron" {
}


resource "spheron_instance" "instance_test" {
  image         = "crccheck/hello-world"
  tag           = "latest"
  cluster_name  = "tf_test"
  region        = "any"
  machine_image = "Ventus Small"

  # args     = ["arg"]
  # commands = ["command"]

  ports = [
    {
      container_port = 8000
    }
  ]

  # health_check = {
  #   path = "/"
  #   port = 8000
  # }
}

output "instance_id" {
  value = spheron_instance.instance_test.id
}

resource "spheron_domain" "domain_test" {
  name = "test.com"
  type = "domain"

  instance_port = spheron_instance.instance_test.ports[0].container_port
  instance_id   = spheron_instance.instance_test.id
}

resource "spheron_marketplace_instance" "instance_market_test" {
  name          = "MongoDB"
  machine_image = "Ventus Nano"
}
