package grid_amqp

import (
	"log"
)

type Cmds struct {
}

func (c *Cmds) Ka(msg *AMQP_Message) error {
	log.Printf("Processing Ka cmd..")
	return nil
}

func (c *Cmds) Add(msg *AMQP_Message) error {
	log.Printf("Processing Add cmd..")
	log.Printf("Adding: %+v", (*msg.Msg1))
	return nil
}

func (c *Cmds) Ver(msg *AMQP_Message) error {
	log.Printf("Processing Ver cmd..")
	log.Printf("Version: %s", (*msg.Params)["ver"])
	return nil
}
