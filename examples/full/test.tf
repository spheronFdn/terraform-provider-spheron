terraform {
  required_providers {
    spheron = {
      version = "1.0.0"
      source  = "spheron/spheron"
    }
  }
}

provider "spheron" {
  token = ""
}


resource "spheron_instance" "instance_test2" {
  image         = "crccheck/hello-world"
  tag           = "latest"
  cluster_name  = "tf_test"
  region        = "any"
  machine_image = "Ventus Small"

  args     = ["arg"]
  commands = ["command"]

  ports = [
    {
      container_port = 8000
      exposed_port   = 80
    }
  ]
  env = [
    {
      key   = "k",
      value = "v"
    }
  ]

  health_check = {
    path = "/"
    port = 8000
  }
}

output "instance_id" {
  value = spheron_instance.instance_test.id
}

resource "spheron_domain" "domain_test" {
  name = "test.com"
  type = "domain"

  instance_port = spheron_instance.instance_test2.ports[0].container_port
  instance_id   = spheron_instance.instance_test2.id
}

resource "spheron_instance" "instance_test2" {
  image         = "crccheck/hello-world"
  tag           = "latest"
  cluster_name  = "tf_test"
  region        = "any"
  machine_image = "Ventus Small"

  args     = ["arg"]
  commands = ["command"]

  ports = [
    {
      container_port = 8000
    }
  ]
  env = [
    {
      key   = "k",
      value = "v"
    }
  ]

  health_check = {
    path = "/"
    port = 8000
  }
}

output "instance_id" {
  value = spheron_instance.instance_test.id
}

resource "spheron_domain" "domain_test" {
  name = "test.com"
  type = "domain"

  instance_port = spheron_instance.instance_test2.ports[0].container_port
  instance_id   = spheron_instance.instance_test2.id
}

resource "spheron_marketplace_instance" "instance_market_test" {
  name          = "Postgres"
  machine_image = "Ventus Nano"

  env = [
    {
      key   = "POSTGRES_PASSWORD"
      value = "passSecrettt"
    },
    {
      key   = "POSTGRES_USER"
      value = "admin"
    },
    {
      key   = "POSTGRES_DB"
      value = "myDB"
    }
  ]

  region = "any"
}

resource "spheron_marketplace_instance" "instance_IPFS_test" {
  name          = "IPFS"
  machine_image = "Ventus Nano"
  region        = "any"
}

resource "spheron_domain" "domain_test_ipfs" {
  name = "test.com"
  type = "domain"

  instance_port = spheron_marketplace_instance.instance_IPFS_test.ports[0].container_port
  instance_id   = spheron_marketplace_instance.instance_IPFS_test.id
}
