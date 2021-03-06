package service

import (
	"errors"
	"fmt"
	"github.com/ardiantirta/go-user-management/helper"
	"github.com/ardiantirta/go-user-management/models"
	"github.com/ardiantirta/go-user-management/services/auth"
	"github.com/dgrijalva/jwt-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/spf13/viper"
	"time"
)

type AuthService struct {
	AuthRepository auth.Repository
}

func (a *AuthService) Register(req *models.RegisterForm) (map[string]interface{}, error) {
	user, verificationCode, err := a.AuthRepository.Register(req)
	if err != nil {
		return helper.ErrorMessage(0, err.Error()), err
	}

	sg := new(models.SendGridEmail)
	sg.From = mail.NewEmail("User Example 1", "user1@example.com")
	sg.To = mail.NewEmail(user.FullName, user.Email)
	sg.Subject = "Email Verification: user-management-go"
	sg.PlainContent = "Please verify your email"
	sg.HtmlContent = fmt.Sprintf(`<a href="http://localhost:3000/auth/verification/%s">email verification</a>`, verificationCode.Code)
	if err := helper.SendVerificationByEmail(sg); err != nil {
		return helper.ErrorMessage(0, err.Error()), err
	}

	//go func() {
	//	_ = helper.SendVerificationByEmail(user, verificationCode.Code)
	//}()

	mapResponse := map[string]interface{}{"status": true}
	return mapResponse, nil
}

func (a *AuthService) Verification(params map[string]interface{}) (map[string]interface{}, error) {
	err := a.AuthRepository.Verification(params)
	if err != nil {
		return helper.ErrorMessage(0, err.Error()), err
	}

	mapResponse := map[string]interface{}{"status": true}
	return mapResponse, nil
}

func (a *AuthService) SendVerificationCode(params map[string]interface{}) (map[string]interface{}, error) {
	emailType := params["type"].(string)
	recipient := params["recipient"].(string)

	user, verificationCode, err := a.AuthRepository.SendVerificationCode(recipient)
	if err != nil {
		return helper.ErrorMessage(0, err.Error()), err
	}

	sg := new(models.SendGridEmail)
	sg.From = mail.NewEmail("User Example 1", "user1@example.com")
	sg.To = mail.NewEmail(user.FullName, user.Email)
	switch emailType {
	case "email.verify":
		sg.Subject = "Email Verification: user-management-go"
	default:
		sg.Subject = "hello from user-management-go"
	}
	sg.PlainContent = "Please verify your email"
	sg.HtmlContent = fmt.Sprintf(`<a href="http://localhost:3000/auth/verification/%s">verify here</a>`, verificationCode)
	if err := helper.SendVerificationByEmail(sg); err != nil {
		return helper.ErrorMessage(0, err.Error()), err
	}

	return map[string]interface{}{"status": true}, nil
}

func (a *AuthService) Login(email, password string) (map[string]interface{}, error) {
	response, err := a.AuthRepository.Login(email, password)
	if err != nil {
		return helper.ErrorMessage(0, err.Error()), err
	}

	return response, nil
}

func (a *AuthService) TwoFactorAuthVerify(id int, code string) (map[string]interface{}, error) {
	currentUser, err := a.AuthRepository.FetchUserByID(id)
	if err != nil {
		return helper.ErrorMessage(0, err.Error()), err
	}

	if currentUser.TFACode != code {
		err := errors.New("wrong code")
		return helper.ErrorMessage(0, err.Error()), err
	}

	userToken, err := a.AuthRepository.FetchUserTokenByUserID(id)
	if err != nil {
		return helper.ErrorMessage(0, err.Error()), err
	}

	isTfa := false
	if currentUser.IsTFA == 1 {
		isTfa = true
	}

	expiredAt := time.Now().UTC().AddDate(0, 0, 7)
	signKey := []byte(viper.GetString("jwt.signkey"))
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, models.CustomClaims{
		ID:    int(currentUser.ID),
		Email: currentUser.Email,
		IsTFA: isTfa,
		Code: code,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expiredAt.Unix(),
		},
	})

	tokenString, _ := token.SignedString(signKey)

	userToken.Token = tokenString
	if err := a.AuthRepository.SaveUserToken(userToken); err != nil {
		return helper.ErrorMessage(0, err.Error()), err
	}

	expiredAtStr := expiredAt.Format(helper.FormatRFC8601)

	return map[string]interface{}{
		"access_token": map[string]interface{} {
			"value": userToken.Token,
			"type": userToken.Type,
			"expired_at": expiredAtStr,
		},
	}, nil
}

func (a *AuthService) TwoFactorAuthByPass(id int, code string) (map[string]interface{}, error) {
	currentUser, err := a.AuthRepository.FetchUserByID(id)
	if err != nil {
		return helper.ErrorMessage(0, err.Error()), err
	}

	backupCodes, err := a.AuthRepository.FetchBackUpCodesByUserID(id)
	if err != nil {
		return helper.ErrorMessage(0, err.Error()), err
	}

	validCode := false
	for _, b := range backupCodes {
		if b.Code == code {
			validCode = true
			break
		}
	}

	if validCode == false {
		err := errors.New("wrong code, try again")
		return helper.ErrorMessage(0, err.Error()), err
	}

	userToken, err := a.AuthRepository.FetchUserTokenByUserID(id)
	if err != nil {
		return helper.ErrorMessage(0, err.Error()), err
	}

	isTfa := false
	if currentUser.IsTFA == 1 {
		isTfa = true
	}

	expiredAt := time.Now().UTC().AddDate(0, 0, 7)
	signKey := []byte(viper.GetString("jwt.signkey"))
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, models.CustomClaims{
		ID:    int(currentUser.ID),
		Email: currentUser.Email,
		IsTFA: isTfa,
		Code: currentUser.TFACode,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expiredAt.Unix(),
		},
	})

	tokenString, _ := token.SignedString(signKey)

	userToken.Token = tokenString
	if err := a.AuthRepository.SaveUserToken(userToken); err != nil {
		return helper.ErrorMessage(0, err.Error()), err
	}

	expiredAtStr := expiredAt.Format(helper.FormatRFC8601)

	return map[string]interface{}{
		"access_token": map[string]interface{} {
			"value": userToken.Token,
			"type": userToken.Type,
			"expired_at": expiredAtStr,
		},
	}, nil
}

func (a *AuthService) ForgotPassword(email string) (map[string]interface{}, error) {
	user, token, err := a.AuthRepository.ForgotPassword(email)
	if err != nil {
		return helper.ErrorMessage(0, err.Error()), err
	}

	sg := new(models.SendGridEmail)
	sg.From = mail.NewEmail("User Example 1", "user1@example.com")
	sg.To = mail.NewEmail(user.FullName, user.Email)
	sg.Subject = "Forgot Password Token"
	sg.PlainContent = "here is you reset password token"
	sg.HtmlContent = token
	if err := helper.SendVerificationByEmail(sg); err != nil {
		return helper.ErrorMessage(0, err.Error()), err
	}

	//go func() {
	//	_ = helper.SendVerificationByEmail(user, verificationCode.Code)
	//}()

	mapResponse := map[string]interface{}{"status": true, "token": token}
	return mapResponse, nil
}

func (a *AuthService) ResetPassword(email, password string) (map[string]interface{}, error) {
	response, err := a.AuthRepository.ResetPassword(email, password)
	if err != nil {
		return helper.ErrorMessage(0, err.Error()), err
	}

	return response, nil
}

func NewAuthService(authRepository auth.Repository) auth.Service {
	return &AuthService{
		AuthRepository: authRepository,
	}
}
