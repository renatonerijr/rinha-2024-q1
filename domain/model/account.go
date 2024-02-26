package model

import (
	"errors"

	"github.com/asaskevich/govalidator"
)

type Account struct {
	id     int32
	saldo  int32
	limite int32
}

type AccountRepositoryInterface interface {
	Register(account *Account) error
	Save(account *Account) error
	Find(accountId int32) (*Account, error)
}

func (account *Account) isValid() error {
	_, err := govalidator.ValidateStruct(account)
	if err != nil {
		return err
	}
	return nil
}

func (account *Account) removeSaldo(valor int32) error {
	if account.saldo-valor <= -account.limite {
		return errors.New("balance cannot be lower than limit")
	}
	account.saldo -= valor
	return nil
}

func (account *Account) removeLimite(valor int32) error {
	if account.limite-valor <= 0 {
		return errors.New("limit cannot be zero or lower")
	}
	account.limite -= valor
	return nil
}

func NewAccount(id int32, saldo int32, limite int32) (*Account, error) {
	account := Account{
		id:     id,
		saldo:  saldo,
		limite: limite,
	}

	err := account.isValid()
	if err != nil {
		return nil, err
	}

	return &account, nil

}
