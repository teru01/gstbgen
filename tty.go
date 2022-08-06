package main

import (
	"fmt"
	"log"

	"github.com/mattn/go-tty"
)

func openTTY(pressQ chan struct{}) error {
	t, err := tty.Open()
	if err != nil {
		// TODO: テスト時にttyが割り当たらない
		fmt.Println("failed to open tty: %w", err)
		return nil
	}
	go func(t *tty.TTY) {
		defer t.Close()
		for {
			r, err := t.ReadRune()
			if err != nil {
				log.Println(err)
			}
			if r == 'q' {
				close(pressQ)
				break
			}
			log.Printf("Press q to quit (pressed %v).\n", string(r))
		}
	}(t)
	return nil
}
