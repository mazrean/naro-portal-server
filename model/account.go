package model

import (
	"fmt"
	"net/http"
	"time"
	"unicode/utf8"

	"github.com/labstack/echo"
	"github.com/labstack/echo-contrib/session"
	"github.com/pborman/uuid"
	"golang.org/x/crypto/bcrypt"
)

//User Userの構造体
type User struct {
	UserName     string `json:"userName,omitempty"  db:"name"`
	UserPassword string `json:"userPassword,omitempty"  db:"password"`
}

//Me GetWhoAmIHandlerの構造体
type Me struct {
	UserName string `json:"userName,omitempty" db:"name"`
}

type TweetID struct {
	TweetID string `db:"tweet_ID"`
}

//PostLoginHandler POST /login ログイン
func PostLoginHandler(c echo.Context) error {
	req := User{}
	c.Bind(&req)

	user := User{}
	err := Db.Get(&user, "SELECT name,password FROM user WHERE name=?", req.UserName)
	if err != nil {
		return c.String(http.StatusInternalServerError, "username is wrong or something wrong in getting user`s information")
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.UserPassword), []byte(req.UserPassword))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return c.NoContent(http.StatusForbidden)
		}
		return c.NoContent(http.StatusInternalServerError)
	}

	sess, err := session.Get("sessions", c)
	if err != nil {
		fmt.Println(err)
		return c.String(http.StatusInternalServerError, "something wrong in getting session")
	}
	sess.Values["UserName"] = req.UserName
	var userID string
	err = Db.Get(&userID, "SELECT ID FROM user WHERE name=?", req.UserName)
	if err != nil {
		return c.String(http.StatusInternalServerError, "username is wrong or something wrong in getting user`s information")
	}
	sess.Values["UserID"] = userID
	sess.Values["ClientID"] = uuid.New()
	sess.Values["LastReloadTime"] = time.Now()
	sess.Save(c.Request(), c.Response())

	return c.NoContent(http.StatusOK)
}

//PostLogoutHandler Post /logout ログアウト
func PostLogoutHandler(c echo.Context) error {
	sess, err := session.Get("sessions", c)
	if err != nil {
		fmt.Println(err)
		return c.String(http.StatusInternalServerError, "something wrong in getting session")
	}

	sess.Values["UserName"] = nil
	err = sess.Save(c.Request(), c.Response())
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}
	return c.NoContent(http.StatusOK)
}

//CheckLogin ログイン確認
func CheckLogin(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		sess, err := session.Get("sessions", c)
		if err != nil {
			fmt.Println(err)
			return c.String(http.StatusInternalServerError, "something wrong in getting session")
		}

		if sess.Values["UserName"] == nil {
			return c.String(http.StatusForbidden, "please login")
		}
		c.Set("UserName", sess.Values["UserName"].(string))
		c.Set("UserID", sess.Values["UserID"].(string))

		return next(c)
	}
}

//PostSignUpHandler Post /signup サインアップ
func PostSignUpHandler(c echo.Context) error {
	req := User{}
	c.Bind(&req)

	var userID string
	Db.Get(&userID, "SELECT ID FROM user WHERE name=?", req.UserName)
	if userID != "" {
		return c.String(http.StatusBadRequest, "ユーザーが既に存在しています")
	}

	if utf8.RuneCountInString(req.UserPassword) < 8 {
		return c.String(http.StatusBadRequest, "パスワードは8文字以上です")
	}

	hashedPass, err := bcrypt.GenerateFromPassword([]byte(req.UserPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("bcrypt generate error: %v", err))
	}

	_, err = Db.Exec("INSERT INTO user (name,ID,password) VALUES (?, ?,?)", req.UserName, uuid.New(), hashedPass)
	if err != nil {
		return c.String(http.StatusInternalServerError, fmt.Sprintf("db error: %v", err))
	}

	sess, err := session.Get("sessions", c)
	if err != nil {
		fmt.Println(err)
		return c.String(http.StatusInternalServerError, "something wrong in getting session")
	}
	sess.Values["UserName"] = req.UserName
	err = Db.Get(&userID, "SELECT ID FROM user WHERE name=?", req.UserName)
	if err != nil {
		return c.String(http.StatusInternalServerError, "username is wrong or something wrong in getting user`s information")
	}
	sess.Values["UserID"] = userID
	sess.Values["ClientID"] = uuid.New()
	sess.Values["LastReloadTime"] = time.Now()
	err = sess.Save(c.Request(), c.Response())
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusCreated)
}

//DeleteAccountHandler Delete /account
func DeleteAccountHandler(c echo.Context) error {
	sess, err := session.Get("sessions", c)
	if err != nil {
		fmt.Println(err)
		return c.String(http.StatusInternalServerError, "something wrong in getting session")
	}

	_, err = Db.Exec("DELETE FROM pin WHERE user_ID = ?", c.Get("UserID").(string))
	_, err = Db.Exec("DELETE FROM favorite JOIN tweet ON favorite.tweet_ID = tweet.tweet_ID WHERE tweet.user_ID = ? OR favorite.user_ID = ?", c.Get("UserID").(string), c.Get("UserID").(string))
	_, err = Db.Exec("DELETE FROM user WHERE ID = ?", c.Get("UserID").(string))

	sess.Values["UserName"] = nil
	err = sess.Save(c.Request(), c.Response())
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.NoContent(http.StatusOK)
}

//GetWhoAmIHandler Get /whoAmI
func GetWhoAmIHandler(c echo.Context) error {
	return c.JSON(http.StatusOK, Me{
		UserName: c.Get("UserName").(string),
	})
}

//GetUserListHandler Get /userList
func GetUserListHandler(c echo.Context) error {
	userNames := []Me{}
	err := Db.Select(&userNames, "SELECT name FROM user")
	if err != nil {
		return c.NoContent(http.StatusInternalServerError)
	}

	return c.JSON(http.StatusOK, userNames)
}
