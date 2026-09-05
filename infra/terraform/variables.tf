variable "ssh_key_name" {
  type = string
}

variable "admin_ip" {
  description = "Your public IP in CIDR format"
  type        = string
}

variable "region" {
  type    = string
  default = "nyc3"
}