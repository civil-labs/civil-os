env "local" {
  # Point src to the compiled data source
  src = "file://schema/schema.pg.hcl"

  dev = "docker://postgis/17-3.5-alpine/dev"
  migration {
    dir = "file://migrations"
  }
}
