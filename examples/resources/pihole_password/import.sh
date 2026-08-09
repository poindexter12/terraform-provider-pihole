# The admin password is a singleton; the import ID is always "admin-password".
# The password itself cannot be read from the API, so the first plan after
# import shows an update that asserts the configured value.
terraform import pihole_password.admin admin-password
