package main

import (
	"log"

	"github.com/hajimehoshi/ebiten/v2"

	"ebiten-fullstack-template/internal/client"
)

func main() {
	ebiten.SetWindowSize(client.ScreenWidth, client.ScreenHeight)
	ebiten.SetWindowTitle("Ebiten Fullstack Template")

	game := client.NewGame()
	if err := ebiten.RunGame(game); err != nil {
		log.Fatal(err)
	}
}
