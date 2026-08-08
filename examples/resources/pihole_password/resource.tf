# Manage the Pi-hole admin password.
#
# The provider must authenticate with the CURRENT password to change it. To
# rotate the admin password in a single apply, authenticate the provider with
# a Pi-hole app password (Settings > Web interface/API > Configure app
# password), which stays valid across admin password changes:
#
#   provider "pihole" {
#     url      = "https://pihole.domain.com"
#     password = var.pihole_app_password # unaffected by rotation
#   }
#
# Without an app password, rotation takes two applies: first apply the new
# password below while the provider still authenticates with the old one,
# then update the provider credential.
#
# Destroying this resource removes it from Terraform state but leaves the
# password set (Pi-hole's only alternative is an empty password, which
# disables authentication entirely).
resource "pihole_password" "admin" {
  password = var.pihole_admin_password
}

# Other resources do not depend on the password implicitly; use depends_on
# when the password must be set first (e.g. on a freshly provisioned Pi-hole).
resource "pihole_dns_record" "example" {
  domain = "example.lan"
  ip     = "192.168.1.10"

  depends_on = [pihole_password.admin]
}
