package grid_amqp

import (
	//IP "3grid/ip"
	RT "3grid/route"
	//G "3grid/tools/globals"
	"log"
	"errors"
	"fmt"
)

type Cmds struct {
}

func (c *Cmds) Ka(msg *AMQP_Message) error {
	log.Printf("Processing Ka cmd..")
	return nil
}

func (c *Cmds) Add(msg *AMQP_Message) (err error) {
	log.Printf("Processing Add cmd..")
	log.Printf("Adding: %+v", (*msg.Msg1))
	return nil
}

func (c *Cmds) Ver(msg *AMQP_Message) error {
	log.Printf("Processing Ver cmd..")
	log.Printf("Version: %s", (*msg.Params)["ver"])
	return nil
}

func (c *Cmds) Update(msg *AMQP_Message) (err error) {
	defer func(){
		if pan := recover(), pan != nil{
			msg := fmt.Sprintf("%s", pan)
			err = errors.New(msg)
		}
	}()
	db_type := msg.Object
	datas := map[string][string][]string{db_type: *msg.Msg1}
	*RT.Chan <- datas
	return err
}
