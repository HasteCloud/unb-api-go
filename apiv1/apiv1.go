package apiv1

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type UserData struct {
	token  string
	client *http.Client
}

type errorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

type timeoutResponse struct {
	Message    string        `json:"message"`
	RetryAfter time.Duration `json:"retry_after"`
}

type check struct {
	Ping time.Duration
	Up   bool
}

type UserObj struct {
	Rank          int    `json:"rank"`
	UserID        string `json:"user_id"`
	Cash          int    `json:"cash"`
	CashInfinite  bool   `json:"infinite_cash"`
	CashNinfinite bool   `json:"n-infinite_cash"`
	Bank          int    `json:"bank"`
	BankInfinite  bool   `json:"infinite_bank"`
	BankNinfinite bool   `json:"n-infinite_bank"`
	Total         int    `json:"total"`
	Infinite      bool   `json:"infinite_total"`
	Ninfinite     bool   `json:"n-infinite_total"`
}

type userObjwReason struct {
	UserID        string `json:"user_id"`
	Cash          int    `json:"cash"`
	CashInfinite  bool   `json:"infinite_cash"`
	CashNinfinite bool   `json:"n-infinite_cash"`
	Bank          int    `json:"bank"`
	BankInfinite  bool   `json:"infinite_bank"`
	BankNinfinite bool   `json:"n-infinite_bank"`
	Reason        string `json:"reason"`
}

type userObjRaw struct {
	Rank   interface{} `json:"rank"`
	UserID interface{} `json:"user_id"`
	Cash   interface{} `json:"cash"`
	Bank   interface{} `json:"bank"`
	Total  interface{} `json:"total"`
}

type userObjPut struct {
	Cash   interface{} `json:"cash,omitempty"`
	Bank   interface{} `json:"bank,omitempty"`
	Reason interface{} `json:"reason,omitempty"`
}

func (u *UserData) Request(protocol, url string, payload []byte) ([]byte, error) {
	b := bytes.NewBuffer(payload)
	req, err := http.NewRequest(protocol, "https://unbelievable.pizza/api/v1"+url, b)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", u.token)
	resp, err := u.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respo, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == 429 {
		err := timeoutResponse{}
		srsly := json.Unmarshal(respo, &err)
		if srsly != nil {
			// This is a srsly bad error -_-
			panic(err)
		}
		return respo, fmt.Errorf("%v Retry after: %s", err.Message, err.RetryAfter)
	}
	// Bit hacky, test if the response contains the error body
	if strings.Contains(string(respo), "error") {
		err := errorResponse{}
		srsly := json.Unmarshal(respo, &err)
		if srsly != nil {
			// This is a srsly bad error -_-
			panic(err)
		}
		return respo, fmt.Errorf("%v (%v)", err.Error, err.Message)
	}
	return respo, err
}

func fixTypesToStruct(data []byte) (UserObj, error) {
	balUser := UserObj{}
	var objmap map[string]interface{}
	err := json.Unmarshal(data, &objmap)
	if err != nil {
		return UserObj{}, err
	}
	_, totalIsString := objmap["total"].(string)
	if totalIsString {
		switch x := objmap["total"]; x {
		case "Infinity":
			objmap["total"] = 0
			objmap["infinite_total"] = true
		case "-Infinity":
			objmap["total"] = -0
			objmap["n-infinite_total"] = true
		default:
			objmap["total"], _ = strconv.ParseInt(objmap["total"].(string), 0, 64)
		}

	}
	_, cashIsString := objmap["cash"].(string)
	if cashIsString {
		switch x := objmap["cash"]; x {
		case "Infinity":
			objmap["cash"] = 0
			objmap["infinite_cash"] = true
		case "-Infinity":
			objmap["cash"] = -0
			objmap["n-infinite_cash"] = true
		default:
			objmap["cash"], _ = strconv.ParseInt(objmap["cash"].(string), 0, 64)
		}
	}
	_, bankIsString := objmap["bank"].(string)
	if bankIsString {
		switch x := objmap["bank"]; x {
		case "Infinity":
			objmap["bank"] = 0
			objmap["infinite_bank"] = true
		case "-Infinity":
			objmap["bank"] = -0
			objmap["n-infinite_bank"] = true
		default:
			objmap["bank"], _ = strconv.ParseInt(objmap["bank"].(string), 0, 64)
		}
	}
	_, rankIsString := objmap["rank"].(string)
	if rankIsString {
		objmap["rank"], _ = strconv.ParseInt(objmap["rank"].(string), 0, 64)
	}

	b, err := json.Marshal(objmap)
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal([]byte(b), &balUser)
	if err != nil {
		return UserObj{}, err
	}

	return balUser, err
}

func New(token string) UserData {
	client := &http.Client{}
	u := UserData{token, client}
	return u
}

func Custom(token string, client *http.Client) UserData {
	u := UserData{token, client}
	return u
}

func (u *UserData) Check() (check, error) {
	start := time.Now()
	data, err := u.Request("GET", "", nil)
	elapsed := time.Since(start)
	if err != nil {
		// because we never know how long.
		if strings.Contains(string(data), `{"message":"You are being rate limited.","retry_after"`) {
			return check{time.Since(time.Now()), true}, err
		}
		switch x := string(data); x {
		case `{"error":"404: Not found"}`:
			return check{elapsed, true}, nil

		case `{"error":"401: Unauthorized"}`:
			return check{elapsed, true}, errors.New("401 Unauthorized (Check your token)")

		default:
			return check{time.Since(time.Now()), false}, err
		}
	}
	return check{time.Since(time.Now()), false}, errors.New("Cannot connect to API url")
}

func (u *UserData) GetBalance(guild, user string) (UserObj, error) {
	data, err := u.Request("GET", fmt.Sprintf("/guilds/%v/users/%v", guild, user), nil)
	if err != nil {
		return UserObj{}, err
	}
	userBal, err := fixTypesToStruct(data)
	if err != nil {
		return UserObj{}, err
	}
	return userBal, err
}

func (u *UserData) SetBalance(guild, user string, cash, bank, reason interface{}) (UserObj, error) {
	var payloadTypes = make(map[string]interface{})
	if cash != nil {
		switch x := cash; x.(type) {
		case string:
			if cash == "Infinity" {
				payloadTypes["cash"] = "Infinity"
			} else if cash == "-Infinity" {
				payloadTypes["cash"] = "-Infinity"
			}
		case int:
			payloadTypes["cash"] = cash
		}
	}
	if bank != nil {
		switch x := bank; x.(type) {
		case string:
			if bank == "Infinity" {
				payloadTypes["bank"] = "Infinity"
			} else if bank == "-Infinity" {
				payloadTypes["bank"] = "-Infinity"
			}
		case int:
			payloadTypes["bank"] = bank
		}
	}
	switch x := reason; x.(type) {
	case string:
		payloadTypes["reason"] = reason
	case nil:
		payloadTypes["reason"] = "No reason provided."
	}
	value, err := json.Marshal(payloadTypes)
	if err != nil {
		return UserObj{}, err
	}
	data, err := u.Request("PUT", fmt.Sprintf("/guilds/%v/users/%v", guild, user), value)
	if err != nil {
		return UserObj{}, err
	}
	userBal, err := fixTypesToStruct(data)
	if err != nil {
		return UserObj{}, err
	}
	return userBal, err
}

func (u *UserData) UpdateBalance(guild, user string, cash, bank int, reason interface{}) (UserObj, error) {
	var payloadTypes = make(map[string]interface{})
	payloadTypes["cash"] = cash
	payloadTypes["bank"] = bank
	switch x := reason; x.(type) {
	case string:
		payloadTypes["reason"] = reason
	case nil:
		payloadTypes["reason"] = "No reason provided."
	}
	value, err := json.Marshal(payloadTypes)
	if err != nil {
		return UserObj{}, err
	}
	data, err := u.Request("PATCH", fmt.Sprintf("/guilds/%v/users/%v", guild, user), value)
	if err != nil {
		return UserObj{}, err
	}
	userBal, err := fixTypesToStruct(data)
	if err != nil {
		return UserObj{}, err
	}
	return userBal, err
}

func (u *UserData) Leaderboard(guild string) ([]UserObj, error) {
	var leaderboardRaw []userObjRaw
	var leaderboard []UserObj

	data, err := u.Request("GET", fmt.Sprintf("/guilds/%v/users", guild), nil)
	if err != nil {
		return []UserObj{}, err
	}

	if err := json.Unmarshal(data, &leaderboardRaw); err != nil {
		return []UserObj{}, err
	}
	for _, v := range leaderboardRaw {
		value := fmt.Sprintf(`{"rank":"%v","user_id":"%v","cash":"%v","bank":"%v","total":"%v"}`, v.Rank, v.UserID, v.Cash, v.Bank, v.Total)
		user, err := fixTypesToStruct([]byte(value))
		if err != nil {
			return []UserObj{}, err
		}
		leaderboard = append(leaderboard, user)
	}

	if err != nil {
		return []UserObj{}, err
	}
	return leaderboard, err
}
