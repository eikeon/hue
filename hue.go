package hue

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

type Light struct {
	Name  string `json:"name"`
	State State  `json:"state"`
}

type State struct {
	On        bool       `json:"on"`
	Bri       uint8      `json:"bri"`
	Alert     string     `json:"alert"`
	Effect    string     `json:"effect"`
	ColorMode string     `json:"colormode"`
	Hue       uint16     `json:"hue"`
	Sat       uint8      `json:"sat"`
	Xy        [2]float32 `json:"xy"`
	Ct        uint16     `json:"ct"`
	Reachable bool       `json:"reachable"`
}

type Group struct {
	Name   string   `json:"name"`
	Lights []string `json:"lights"`
	Action State    `json:"action"`
}

type Datastore struct {
	Lights map[string]Light `json:"lights"`
	Groups map[string]Group `json:"groups"`
	//Config
	//Schedules
	//Scene
}

type Hue struct {
	Host     string
	Username string

	// a copy of the hue datastore from the bridge
	Datastore Datastore

	// errors returned from the last call to the bridge
	Errors []struct {
		Error struct {
			Type        int
			Address     string
			Description string
		}
	}
}

type bridge struct {
	Id                string
	Internalipaddress string
	Macaddress        string
}
type bridgeResponse []bridge

func (h *Hue) getHost() (string, error) {
	if h.Host == "" {
		if response, err := http.Get("http://www.meethue.com/api/nupnp"); err == nil {
			dec := json.NewDecoder(response.Body)
			var br bridgeResponse
			if err = dec.Decode(&br); err == nil {
				if len(br) == 1 {
					b := br[0]
					h.Host = b.Internalipaddress
				} else {
					return "", errors.New("bridgeResponse != 1 not yet implemented")
				}
			} else {
				return "", errors.New(fmt.Sprintf("could not decode bridgeResponse: %s", err))
			}
			response.Body.Close()
		} else {
			return "", errors.New(fmt.Sprintf("could not get: %s", err))
		}

	}
	return h.Host, nil
}

func (h *Hue) CreateUser(username, devicetype string) error {
	if b, err := json.Marshal(struct {
		Devicetype string `json:"devicetype,omitempty"`
		Username   string `json:"username,omitempty"`
	}{"Marvin", username}); err == nil {
		host, err := h.getHost()
		if err != nil {
			return err
		}
		if r, err := http.NewRequest("POST", "http://"+host+"/api", bytes.NewReader(b)); err == nil {
			if response, err := http.DefaultClient.Do(r); err == nil {
				if body, err := ioutil.ReadAll(response.Body); err == nil {
					response.Body.Close()
					// example: [{"success":{"username": "1234567890"}}]
					var r []map[string]map[string]string
					if err = json.Unmarshal(body, &r); err == nil {
						if len(r) > 0 {
							s, ok := r[0]["success"]
							if ok {
								h.Username = s["username"]
								h.Errors = h.Errors[0:0]
								return nil
							}
						}
						// TODO: implement multiple bridge etc case
						return errors.New("user not autherized?")
					} else {
						if err = json.Unmarshal(body, &(h.Errors)); err == nil {
							// TODO: check error response
							return errors.New("user not autherized")
						} else {
							log.Println("body:", string(body))
							return errors.New("unexpected response")
						}
					}
				} else {
					return err
				}
			} else {
				return err
			}
		} else {
			return err
		}
	} else {
		log.Fatal("ERROR: json.Marshal: " + err.Error())
	}
	return nil
}

func (h *Hue) GetState() error {
	if h.Username == "" {
		return errors.New("error: no user")
	}
	host, err := h.getHost()
	if err != nil {
		return err
	}
	u := "http://" + host + "/api/" + h.Username
	if response, err := http.Get(u); err == nil {
		body, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return err
		}
		response.Body.Close()
		h.Errors = h.Errors[0:0]
		err = json.Unmarshal(body, &(h.Datastore))
		if err != nil {
			log.Println("WARNING: could not unmarshal datastore:", err)
			err = json.Unmarshal(body, &(h.Errors))
			if err != nil {
				return err
			} else {
				if len(h.Errors) == 1 {
					if h.Errors[0].Error.Type == 1 {
						return errors.New(fmt.Sprintf("%v", h.Errors))
					}
				}
			}
		}
	} else {
		return err
	}
	// get group 0 which is not included in full datastore dump for some reason :(
	if response, err := http.Get(u + "/groups/0"); err == nil {
		dec := json.NewDecoder(response.Body)
		var l Group
		if err = dec.Decode(&l); err == nil {
			h.Datastore.Groups["0"] = l
		} else {
			log.Fatal("could not decode group:", err)
		}
		response.Body.Close()
	} else {
		log.Fatal("could not get group:", err)
	}
	return nil
}

func (h *Hue) Set(address string, value interface{}) error {
	host, err := h.getHost()
	if err != nil {
		return err
	}
	if h.Username == "" {
		return errors.New("error: no user")
	}
	url := "http://" + host + "/api/" + h.Username + address
	b, err := json.Marshal(value)
	if err != nil {
		log.Println("ERROR: json.Marshal: " + err.Error())
		return err
	}
	if r, err := http.NewRequest("PUT", url, bytes.NewReader(b)); err == nil {
		if response, err := http.DefaultClient.Do(r); err == nil {
			response.Body.Close()
			time.Sleep(100 * time.Millisecond)
		} else {
			log.Println("ERROR: client.Do: " + err.Error())
		}
	} else {
		log.Println("ERROR: NewRequest: " + err.Error())
	}
	return nil
}
