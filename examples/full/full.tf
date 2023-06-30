terraform {
  required_providers {
    spheron = {
      version = "1.0.0"
      source  = "spheron/spheron"
    }
  }
}

provider "spheron" {
  token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhcGlLZXkiOiJjZmU0YmM0NTRmMzA0MTg5NDk0NzUyZmUyOGY4YTg3MDEzOWRhMGM3NzkxMjM4YzY2NDY3MGM0Y2NjZDBmNmNmN2ZkNDAwOGNkMDNjN2U1NGExZTA1OWJmYzIyZWYzZDZlN2YyNGNhYmM1NWI1ZDQ5OWY4OGNjOTk4MDc3MjY4OCIsImlhdCI6MTY4NjY2NzIwNiwiaXNzIjoid3d3LnNwaGVyb24ubmV0d29yayJ9.lpC-lSr7kxh1ITq7rUHfT4cd7eBRRMF1wKEVp5WAfaA"
}

# data "spheron_organization" "org" {
# }

resource "spheron_instance" "instance_test" {
  image        = "crccheck/hello-world"
  tag          = "latest"
  cluster_name = "tf_test"
  region       = "any"
  # machine_image = "Ventus Small"

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

  # health_check = {
  #   path = "/test"
  #   port = 8000
  # }

  # persistent_storage = {
  #   class       = "HDD"
  #   mount_point = "/etc/data"
  #   size        = 5
  # }

  storage  = 10
  cpu      = 10
  memory   = 2
  replicas = 1
}

# resource "spheron_marketplace_instance" "instance_IPFS_test" {
#   name = "IPFS"
#   # machine_image = "Ventus Nano"
#   region = "any"

#   # persistent_storage = {
#   #   class       = "HDD"
#   #   mount_point = "/etc/data"
#   #   size        = 2
#   # }

#   storage  = 10
#   cpu      = 1
#   memory   = 2
#   replicas = 1
# }

# resource "spheron_domain" "domain_test_ipfs" {
#   name = "test.com"
#   type = "domain"

#   instance_port = spheron_marketplace_instance.instance_IPFS_test.ports[0].container_port
#   instance_id   = spheron_marketplace_instance.instance_IPFS_test.id
# }
