package grid_amqp

import (
	"log"
)

type Cmds struct {
}

func (this *Cmds) Kaa(msg *AMQP_Message) error {
	log.Printf("Ka cmd: ..")
	return nil
}

func (this *Cmds) Add(msg *AMQP_Message) error {
	log.Printf("Add cmd: ..")
	return nil
}
