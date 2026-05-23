package main

import (
	"os"

	"github.com/vanducng/vd-cli/v2/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
