terraform {
  backend "s3" {
    # Bucket is supplied by task init; endpoint and credentials come from env.
    key          = "terrifi.tfstate"
    region       = "auto"
    use_lockfile = true

    skip_credentials_validation = true
    skip_metadata_api_check     = true
    skip_region_validation      = true
    skip_requesting_account_id  = true
    skip_s3_checksum            = true
    use_path_style              = true
  }
}
