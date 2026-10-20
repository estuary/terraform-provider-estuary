package main

import (
	"context"
	"flag"
	"log"

	"github.com/estuary/terraform-provider-estuary/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var version string = "dev"
var debug bool = false

func main() {
	flag.BoolVar(&debug, "debug", false, "Set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	serveOptions := providerserver.ServeOpts{
		Address: "registry.terraform.io/estuary/estuary",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), serveOptions); err != nil {
		log.Fatal(err.Error())
	}
}
