terraform {
  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 2.0"
    }
  }
}

provider "digitalocean" {}

data "digitalocean_ssh_key" "main" {
  name = var.ssh_key_name
}

resource "digitalocean_droplet" "connect4" {
  name   = "connect4-server"
  region = var.region
  size   = "s-1vcpu-1gb"
  image  = "ubuntu-24-04-x64"

  ssh_keys = [
    data.digitalocean_ssh_key.main.id
  ]

  monitoring = true
}

resource "digitalocean_firewall" "connect4" {
  name = "connect4-firewall"

  droplet_ids = [
    digitalocean_droplet.connect4.id
  ]

  inbound_rule {
    protocol         = "tcp"
    port_range       = "22"
    source_addresses = [var.admin_ip]
  }

  inbound_rule {
    protocol         = "tcp"
    port_range       = "80"
    source_addresses = ["0.0.0.0/0", "::/0"]
  }

  inbound_rule {
    protocol         = "tcp"
    port_range       = "443"
    source_addresses = ["0.0.0.0/0", "::/0"]
  }

  outbound_rule {
    protocol              = "tcp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }

  outbound_rule {
    protocol              = "udp"
    port_range            = "1-65535"
    destination_addresses = ["0.0.0.0/0", "::/0"]
  }
}