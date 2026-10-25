// doubletake-bar is a native tray menu and a CLI status command for
// doubletake (an AirPlay mirroring sender). It talks to a running
// `doubletake -daemonize` over its control socket.
package main

import (
	"fmt"
	"os"

	"github.com/404wolf/doubletake-bar/internal/logging"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var log *zap.Logger

func main() {
	root := &cobra.Command{
		Use:           "doubletake-bar",
		Short:         "Native AirPlay tray menu for doubletake (AirPlay sender)",
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	root.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Print waybar custom-module JSON for the current AirPlay state",
		RunE: func(cmd *cobra.Command, args []string) error {
			log = logging.New("status")
			defer log.Sync()
			runStatus()
			return nil
		},
	})

	root.AddCommand(&cobra.Command{Use: "tray", Aliases: []string{"popup"}, Short: "Run the native AirPlay tray menu", RunE: func(cmd *cobra.Command, args []string) error { return runTray() }})

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
