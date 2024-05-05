package api

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func GetResponse(req *http.Request, target interface{}) error  {
	client := &http.Client{}

	res, err := client.Do(req)

	if err != nil {
		return err
	}

	defer res.Body.Close()

	defer func() {
		if r := recover(); r != nil {
			fmt.Println(r)
		}
	}()

	if err = json.NewDecoder(res.Body).Decode(target); err != nil {
		panic(err)
	}

	return nil
}