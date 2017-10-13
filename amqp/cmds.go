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
	defer func() {
		if pan := recover(); pan != nil {
			msg1 := fmt.Sprintf("%s", pan)
			err = errors.New(msg1)
		}
	}()
	db_type := msg.Object
	*RT.Chan <- map[string]map[string]map[string]map[string][]string{db_type: *msg.Msg1}
	return err
}

func (c *Cmds) Ver(msg *AMQP_Message) error {
	log.Printf("Processing Ver cmd..")
	log.Printf("Version: %s", (*msg.Params)["ver"])
	return nil
}

func (c *Cmds) Update(msg *AMQP_Message) (err error) {
	defer func() {
		if pan := recover(); pan != nil {
			msg1 := fmt.Sprintf("%s", pan)
			err = errors.New(msg1)
		}
	}()
	db_type := msg.Object
	*RT.Chan <- map[string]map[string]map[string]map[string][]string{db_type: *msg.Msg1}
	return err
}

func (c *Cmds) Delete(msg *AMQP_Message) (err error) {
	defer func() {
		if pan := recover(); pan != nil {
			msg1 := fmt.Sprintf("%s", pan)
			erro = errors.New(msg1)
		}
	}()
	db_type := msg.Object
	switch db_type {
	case "Domain":
	case "Cmdb":
		for k1, v1 := range *msg.Msg1 {
			for k2, _ := range v1 {
				(*msg).Msg1[k1][k2] = nil
			}
		}
	case "Routedb":
		for k, _ := range *msg.Msg1 {
			(*msg).Msg1[k] = nil
		}
	}
	*RT.Chan <- map[string]map[string]map[string]map[string][]string{db_type: *msg.Msg1}
	return err
}
