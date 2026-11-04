package main

import (
	"context"
	"flag"
	"log"

	"github.com/estuary/terraform-provider-estuary/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

// Generates the Terraform Registry documentation under docs from the provider schema, examples and templates.
//go:generate go tool tfplugindocs generate --provider-name estuary --rendered-provider-name Estuary

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
