package lostfilmAPI

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

type rInfo struct {
	url, size string
}
type retreInfo struct {
	m          map[string]rInfo
	miniPoster string
}

type Episode struct {
	id      uint
	name    string
	nameEng string
	date    string

	quality string
}
type Season struct {
	id, amountEpisodes      uint
	episodes                []Episode
	poster, detailsId, date string
	full                    bool
	quality                 string
}
type Serial struct {
	id, title, title_eng string
	amountSeasons        uint
	seasons              []Season
	status               bool
	countries            []string
	year                 uint
	coverImage           string
	urls                 []string //site, russianSite, forum, lostfilm
	genreLists           []string
	descriptions         string
	fullDescription      bool
	story                string
	actors               []string
}

func GetSerialsList(page *io.Reader) []Serial {
	var divContentHead, firstDiv, textInA, spanInA, textInSpan bool
	tempSerial := new(Serial)
	tempSerialsList := make([]Serial, 0)
	tokenizer := html.NewTokenizer(*page)
LOOP:
	for {
		ty := tokenizer.Next()
		if ty == html.ErrorToken {
			break
		}
		if ty != html.StartTagToken && ty != html.TextToken && ty != html.EndTagToken {
			continue
		}
		t := tokenizer.Token()
		switch t.Type {
		case html.StartTagToken:
			if t.Data == "div" {
				for _, attr := range t.Attr {
					if attr.Key == "class" && attr.Val == "content_head" {
						divContentHead = true
						continue
					}
					if divContentHead && attr.Key == "class" && attr.Val == "bb" {
						firstDiv = true
						continue
					}
				}
			} else if firstDiv && t.Data == "a" {
				for _, attr := range t.Attr {
					if attr.Key == "href" && strings.HasPrefix(attr.Val, "/browse.php?cat=") {
						tempSerial.id = attr.Val[16:]
					}
				}
				spanInA, textInA = true, true
			} else if spanInA && t.Data == "span" {
				textInSpan, textInA = true, false
			}
		case html.TextToken:
			if textInA {
				tempSerial.title = t.Data
			} else if textInSpan && strings.HasPrefix(t.Data, "(") && strings.HasSuffix(t.Data, ")") {
				tempSerial.title_eng = t.Data[1 : len(t.Data)-1]
			}
		case html.EndTagToken:
			if firstDiv && t.Data == "span" {
				tempSerialsList = append(tempSerialsList, *tempSerial)
				spanInA, textInSpan = false, false
			}
			if firstDiv && t.Data == "div" {
				break LOOP
			}
		}
	}
	return tempSerialsList
}

func Login(login, password string) (*cookiejar.Jar, error) {
	loginUrlValues := make(url.Values)
	loginUrlValues.Add("login", login)
	loginUrlValues.Add("password", password)
	loginForm, loginErr := http.PostForm("http://login1.bogi.ru/login.php?referer=http://www.lostfilm.tv/", loginUrlValues)
	if loginErr != nil {
		return nil, loginErr
	}

	tokenizer := html.NewTokenizer(loginForm.Body)
	var urlForm, tempValue string
	urlValues := make(url.Values)
	for {
		boolValue := false
		ty := tokenizer.Next()
		if ty == html.ErrorToken {
			break
		}

		if ty != html.StartTagToken && ty != html.SelfClosingTagToken {
			continue
		}
		t := tokenizer.Token()
		if t.Data == "form" {
			for _, attr := range t.Attr {
				if attr.Key == "action" {
					urlForm = attr.Val
				}
			}
		}
		if t.Data == "input" {
			for _, attr := range t.Attr {
				switch attr.Key {
				case "name":
					if boolValue {
						urlValues.Add(attr.Val, tempValue)
					}
					boolValue = !boolValue
					tempValue = attr.Val
				case "value":
					if boolValue {
						urlValues.Add(tempValue, attr.Val)
					}
					boolValue = !boolValue
					tempValue = attr.Val
				}
			}
		}
	}
	cookieJar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: cookieJar}
	_, err := client.PostForm(urlForm, urlValues)
	if err != nil {
		return nil, err
	}
	return cookieJar, nil
}

func GetRetreInfo(cookie *cookiejar.Jar, id, s, ep string) (*retreInfo, error) {
	client := &http.Client{Jar: cookie}
	page, clientErr := client.Get("http://lostfilm.tv/nrdr.php?c=" + id + "&s=" + s + "&e=" + ep)
	if clientErr != nil {
		return nil, clientErr
	}
	tokenizer := html.NewTokenizer(page.Body)
	var url string
	var nextA, nextSpan, textInSpan, imgIsPoster bool
	tempRetreInfo := new(retreInfo)
	tempRetreInfo.m = make(map[string]rInfo)
	for {
		ty := tokenizer.Next()
		if ty == html.ErrorToken {
			break
		}
		if ty != html.StartTagToken && ty != html.SelfClosingTagToken && ty != html.TextToken {
			continue
		}
		t := tokenizer.Token()
		if t.Type == html.TextToken && textInSpan {
			textInSpan = false
			quality, size := parseSpanText(t.Data)
			tempRetreInfo.m[quality] = rInfo{url, size}
			continue
		}
		if t.Data == "img" {
			for _, attr := range t.Attr {
				if attr.Key == "src" && !imgIsPoster {
					tempRetreInfo.miniPoster = attr.Val
				} else if attr.Key == "align" && attr.Val == "left" {
					imgIsPoster = true
				}
			}
			if !imgIsPoster {
				tempRetreInfo.miniPoster = ""
			}
		} else if t.Data == "a" {
			if nextA {
				nextA = false
				continue
			}
			for _, attr := range t.Attr {
				if attr.Key == "href" {
					url = attr.Val
				}
			}
			nextA = true
			nextSpan = true
		} else if t.Data == "span" && nextSpan {
			textInSpan = true
			nextSpan = false
		}

	}
	return tempRetreInfo, nil
}

func parseSpanText(text string) (quality, size string) {
	s := strings.Split(text, ". ")
	if strings.HasPrefix(s[0], "Видео: ") {
		quality = s[0][12:]
	}
	if strings.HasPrefix(s[1], "Размер: ") && strings.HasSuffix(s[1], ".") {
		size = s[1][14 : len(s[1])-1]
	}
	return quality, size
}
