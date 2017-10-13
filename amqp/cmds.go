package grid_amqp

import (
	RT "3grid/route"
	G "3grid/tools/globals"
	"errors"
	"fmt"
)

type Cmds struct {
}

func (c *Cmds) Ka(msg *AMQP_Message) error {
	G.Outlog(G.LOG_AMQP, fmt.Sprintf("Processing Ka cmd.."))
	return nil
}

func (c *Cmds) Add(msg *AMQP_Message) (err error) {
<<<<<<< HEAD
	defer func() {
		if pan := recover(); pan != nil {
			msg1 := fmt.Sprintf("%s", pan)
			err = errors.New(msg1)
		}
	}()
	db_type := msg.Object
	*RT.Chan <- map[string]map[string]map[string]map[string][]string{db_type: *msg.Msg1}
	return err
=======
	G.Outlog(G.LOG_AMQP, fmt.Sprintf("Processing Add cmd.."))
	G.Outlog(G.LOG_AMQP, fmt.Sprintf("Adding: %+v", (*msg.Msg1)))
	return nil
>>>>>>> a4b0140e6c64ec7505e993a92d1a5b177be74e47
}

func (c *Cmds) Ver(msg *AMQP_Message) error {
	G.Outlog(G.LOG_AMQP, fmt.Sprintf("Processing Ver cmd.."))
	G.Outlog(G.LOG_AMQP, fmt.Sprintf("Version: %s", (*msg.Params)["ver"]))
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
