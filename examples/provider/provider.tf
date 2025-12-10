terraform {
  required_providers {
    pihole = {
      source  = "terraform.local/local/pihole"
      version = "0.0.1"
    }
  }
}

provider "pihole" {
  url      = "https://pihole.domain.com" # PIHOLE_URL
  password = var.pihole_password         # PIHOLE_PASSWORD
}
