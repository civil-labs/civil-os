env "local" {
  # Point src to the compiled data source
  src = "file://schema/schema.pg.hcl"

  dev = "docker://postgis/18-3.6/dev"

  migration {
    dir = "file://migrations"
  }
}
