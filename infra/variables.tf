variable "do_token" {
  description = "DigitalOcean API token"
  type        = string
  sensitive   = true
}

variable "ssh_key_name" {
  description = "Name of the SSH key already registered in DigitalOcean (doctl compute ssh-key list)"
  type        = string
}

variable "droplet_name" {
  description = "Hostname for the droplet"
  type        = string
  default     = "tusker"
}

variable "region" {
  description = "DigitalOcean region slug (doctl compute region list)"
  type        = string
  default     = "nyc3"
}

variable "droplet_size" {
  description = "Droplet size slug (doctl compute size list)"
  type        = string
  default     = "s-1vcpu-1gb"
}

variable "environment" {
  description = "Environment tag applied to the droplet"
  type        = string
  default     = "production"
}
