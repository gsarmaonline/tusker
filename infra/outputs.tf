output "droplet_ip" {
  description = "Public IP address of the Tusker droplet"
  value       = digitalocean_droplet.tusker.ipv4_address
}

output "ssh_command" {
  description = "SSH into the droplet"
  value       = "ssh root@${digitalocean_droplet.tusker.ipv4_address}"
}

output "app_url" {
  description = "Tusker API base URL (update PORT in /etc/tusker/tusker.env if changed)"
  value       = "http://${digitalocean_droplet.tusker.ipv4_address}:8080"
}
