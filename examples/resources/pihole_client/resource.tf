# Manage a Pi-hole client by IP address
resource "pihole_client" "living_room_tv" {
  client  = "192.168.1.100"
  comment = "Living Room TV"
}

# Manage a Pi-hole client by MAC address
resource "pihole_client" "laptop" {
  client  = "AA:BB:CC:DD:EE:FF"
  comment = "Work Laptop"
}

# Manage a Pi-hole client by hostname
resource "pihole_client" "server" {
  client  = "homeserver.local"
  comment = "Home Server"
}
