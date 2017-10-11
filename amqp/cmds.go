package grid_amqp

import (
	//IP "3grid/ip"
	RT "3grid/route"
	//G "3grid/tools/globals"
	"errors"
	"fmt"
	"log"
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
<<<<<<< HEAD
	defer func(){
		if pan := recover(), pan != nil{
=======
	defer func() {
		if pan := recover(); pan != nil {
>>>>>>> 2eed699998a5dced53b2f6433cd42e8597d835ce
			msg := fmt.Sprintf("%s", pan)
			err = errors.New(msg)
		}
	}()
	db_type := msg.Object
	*RT.Chan <- map[string]map[string][]string{db_type: *msg.Msg1}

	return err
}
