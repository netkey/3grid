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
			err = errors.New(msg1)
		}
	}()
	db_type := msg.Object
	switch db_type {
	case AMQP_OBJ_DOMAIN:
		for k1, v1 := range *msg.Msg1 {
			for k2, v2 := range v1 {
				for k3, _ := range v2 {
					(*msg.Msg1)[k1][k2][k3] = nil
				}
			}
		}
	case AMQP_OBJ_CMDB:
		for k1, v1 := range *msg.Msg1 {
			for k2, _ := range v1 {
				(*msg.Msg1)[k1][k2] = nil
			}
		}
	case AMQP_OBJ_ROUTE:
		for k, _ := range *msg.Msg1 {
			(*msg.Msg1)[k] = nil
		}
	}
	*RT.Chan <- map[string]map[string]map[string]map[string][]string{db_type: *msg.Msg1}
	return err
}
