package repository

import (
	"fmt"
	"rinha/domain/model"

	"gorm.io/gorm"
)

type AccountRepositoryDb struct {
	Db *gorm.DB
}

func (a *AccountRepositoryDb) Register(account *model.Account) error {
	err := a.Db.Create(account).Error

	if err != nil {
		return err
	}

	return nil
}

func (a *AccountRepositoryDb) Save(account *model.Account) error {
	err := a.Db.Save(account).Error

	if err != nil {
		return err
	}

	return nil
}

func (a *AccountRepositoryDb) Find(id string) (*model.Account, error) {
	var account model.Transaction

	a.Db.First(&account, "id = ?", id)

	if account.id == "" {
		return nil, fmt.Errorf("no key was found")
	}

	return &account, nil
}
