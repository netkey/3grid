package grid_amqp

import (
	"log"
)

type Cmds struct {
}

func (this *Cmds) Ka(msg *AMQP_Message) error {
	log.Printf("Processing Ka cmd..")
	return nil
}

func (this *Cmds) Add(msg *AMQP_Message) error {
	log.Printf("Processing Add cmd..")
	return nil
}
