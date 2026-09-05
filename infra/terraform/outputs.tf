output "server_ip" {
  value = digitalocean_droplet.connect4.ipv4_address
}