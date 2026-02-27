package main

import (
	"token_widget/config"
	"token_widget/ui"
)

func main() {
	cfg := config.Load()
	ui.Run(cfg)
}
