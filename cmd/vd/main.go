package main

import (
	"os"

	"github.com/vanducng/vd-cli/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
