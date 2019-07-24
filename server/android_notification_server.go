// Copyright (c) 2015 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/appleboy/go-fcm"
	"github.com/kyokomi/emoji"
)

type AndroidNotificationServer struct {
	AndroidPushSettings AndroidPushSettings
}

func NewAndroideNotificationServer(settings AndroidPushSettings) NotificationServer {
	return &AndroidNotificationServer{AndroidPushSettings: settings}
}

func (me *AndroidNotificationServer) Initialize() bool {
	LogInfo(fmt.Sprintf("Initializing Android notification server for type=%v", me.AndroidPushSettings.Type))

	if len(me.AndroidPushSettings.AndroidApiKey) == 0 {
		LogError("Android push notifications not configured.  Missing AndroidApiKey.")
		return false
	}

	return true
}

func (me *AndroidNotificationServer) SendNotification(msg *PushNotification) PushResponse {
	pushType := msg.Type
	data := map[string]interface{}{
		"ack_id":      msg.AckId,
		"type":        pushType,
		"badge":       msg.Badge,
		"channel_id":  msg.ChannelId,
		"team_id":     msg.TeamId,
		"sender_id":   msg.SenderId,
		"sender_name": msg.SenderName,
		"version":     msg.Version,
	}

	if pushType == PUSH_TYPE_MESSAGE {
		data["message"] = emoji.Sprint(msg.Message)
		data["channel_name"] = msg.ChannelName
		data["post_id"] = msg.PostId
		data["root_id"] = msg.RootId
		data["override_username"] = msg.OverrideUsername
		data["override_icon_url"] = msg.OverrideIconUrl
		data["from_webhook"] = msg.FromWebhook
	}

	incrementNotificationTotal(PUSH_NOTIFY_ANDROID, pushType)

	pushDeviceID := msg.DeviceId

	if strings.Contains(pushDeviceID, "huawei:") {
		clientID := me.AndroidPushSettings.HUAWEIAPPID
		clientSecret := me.AndroidPushSettings.HUAWEIAPPSECRET
		accessTokenURL := "https://login.cloud.huawei.com/oauth2/v2/token"

		resp, err := http.Post(accessTokenURL,
			"application/x-www-form-urlencoded",
			strings.NewReader("grant_type=client_credentials&client_secret="+clientSecret+"&client_id="+clientID))
		if err != nil {
			fmt.Println(err)
			return NewErrorPushResponse("huawei: get access_token error")
		}

		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)

		if err != nil {
			// handle error
			fmt.Println(err)
			return NewErrorPushResponse("huawei: read access_token error")
		}
		var dat map[string]interface{}
		if err := json.Unmarshal(body, &dat); err == nil {
			var r http.Request
			t := time.Now()
			fmt.Println("pushDeviceID: ", pushDeviceID)
			token := strings.Replace(pushDeviceID, "android_rn:huawei:", "", 1)
			fmt.Println("token: ", token)
			deviceTokenList := []string{token}
			deviceTokenListBytes, err := json.Marshal(deviceTokenList)
			if err != nil {
				fmt.Println(err)
			}

			data["google.message_id"] = strconv.FormatInt(t.Unix(), 10)
			data["content"] = data["message"]
			data["title"] = msg.SenderName
			var payloadMap = map[string]interface{}{
				"hps": map[string]interface{}{
					"msg": map[string]interface{}{
						"type": 1,
						"body": data,
					},
				},
			}
			payloadBytes, err := json.Marshal(payloadMap)
			if err != nil {
				fmt.Println(err)
			}

			// 如果包含这些字符，华为推送会报错 Anti-Spam: word is forbidden in [body]
			legalPayloadStr := strings.Replace(string(payloadBytes), "jb", "__", -1)
			legalPayloadStr = strings.Replace(legalPayloadStr, "j8", "__", -1)
			legalPayloadStr = strings.Replace(legalPayloadStr, "sm", "__", -1)

			r.ParseForm()
			r.Form.Add("access_token", fmt.Sprint(dat["access_token"]))
			r.Form.Add("nsp_svc", "openpush.message.api.send")
			r.Form.Add("nsp_ts", strconv.FormatInt(t.Unix(), 10))
			r.Form.Add("device_token_list", string(deviceTokenListBytes))
			r.Form.Add("payload", legalPayloadStr)
			bodystr := strings.TrimSpace(r.Form.Encode())

			nspCtxMap := map[string]interface{}{
				"ver":   "1",
				"appId": clientID,
			}

			nspCtxBytes, err := json.Marshal(nspCtxMap)
			if err != nil {
				fmt.Println(err)
			}
			pushURL := "https://api.push.hicloud.com/pushsend.do?nsp_ctx=" + string(nspCtxBytes)
			resp, err := http.Post(pushURL,
				"application/x-www-form-urlencoded",
				strings.NewReader(bodystr))
			if err != nil {
				fmt.Println(err)
			}

			defer resp.Body.Close()

			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				// handle error
				fmt.Println(err)
			}

			// fmt.Println(string(body))

			var result map[string]interface{}
			if err := json.Unmarshal(body, &result); err == nil {
				if result["code"] != "80000000" {
					fmt.Println(string(body))
					fmt.Println(string(payloadBytes))
					return NewErrorPushResponse("huawei:  push error")
				}
			} else {
				fmt.Println(err)
				return NewErrorPushResponse("huawei:  push error")
			}
		} else {
			fmt.Println(err)
		}
	} else if strings.Contains(pushDeviceID, "xiaomi:") {
		pushURL := "https://api.xmpush.xiaomi.com/v3/message/regid"
		appSecret := me.AndroidPushSettings.XIAOMIAPPSECRET
		registrationID := strings.Replace(pushDeviceID, "xiaomi:", "", 1)
		t := time.Now()
		data["google.message_id"] = strconv.FormatInt(t.Unix(), 10)
		var r http.Request
		dataBytes, err := json.Marshal(data)
		if err != nil {
			fmt.Println(err)
		}
		description := ""
		if data["message"] != nil {
			description = data["message"].(string)
		}
		senderName := ""
		if data["sender_name"] != nil {
			senderName = data["sender_name"].(string)
		}

		r.ParseForm()
		r.Form.Add("payload", string(dataBytes))
		r.Form.Add("restricted_package_name", "com.steedos.messenger")
		r.Form.Add("pass_through", "1")
		r.Form.Add("title", senderName)
		r.Form.Add("description", description)
		r.Form.Add("notify_type", "-1")
		r.Form.Add("registration_id", registrationID)

		bodystr := strings.TrimSpace(r.Form.Encode())
		client := &http.Client{}
		req, err := http.NewRequest("POST", pushURL, strings.NewReader(bodystr))
		if err != nil {
			// handle error
			fmt.Println(err)
		}

		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", "key="+appSecret)

		resp, err := client.Do(req)
		if err != nil {
			// handle error
			fmt.Println(err)
		}

		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			// handle error
			fmt.Println(err)
		}

		// fmt.Println(string(body))
		var result map[string]interface{}
		if err := json.Unmarshal(body, &result); err == nil {
			if result["result"] != "ok" {
				fmt.Println(string(body))
				return NewErrorPushResponse("xiaomi:  push error")
			}
		} else {
			fmt.Println(err)
			return NewErrorPushResponse("xiaomi:  push error")
		}

	} else {
		fcmMsg := &fcm.Message{
			To:       msg.DeviceId,
			Data:     data,
			Priority: "high",
		}

		if len(me.AndroidPushSettings.AndroidApiKey) > 0 {

			sender, err := fcm.NewClient(me.AndroidPushSettings.AndroidApiKey)
			if err != nil {
				incrementFailure(PUSH_NOTIFY_ANDROID, pushType, "invalid ApiKey")
				return NewErrorPushResponse(err.Error())
			}

			LogInfo(fmt.Sprintf("Sending android push notification for device=%v and type=%v", me.AndroidPushSettings.Type, msg.Type))

			start := time.Now()
			resp, err := sender.SendWithRetry(fcmMsg, 2)
			observerNotificationResponse(PUSH_NOTIFY_ANDROID, time.Since(start).Seconds())

			if err != nil {
				LogError(fmt.Sprintf("Failed to send FCM push sid=%v did=%v err=%v type=%v", msg.ServerId, msg.DeviceId, err, me.AndroidPushSettings.Type))
				incrementFailure(PUSH_NOTIFY_ANDROID, pushType, "unknown transport error")
				return NewErrorPushResponse("unknown transport error")
			}

			if resp.Failure > 0 {
				fcmError := resp.Results[0].Error

				if fcmError == fcm.ErrInvalidRegistration || fcmError == fcm.ErrNotRegistered || fcmError == fcm.ErrMissingRegistration {
					LogInfo(fmt.Sprintf("Android response failure sending remove code: %v type=%v", resp, me.AndroidPushSettings.Type))
					incrementRemoval(PUSH_NOTIFY_ANDROID, pushType, fcmError.Error())
					return NewRemovePushResponse()
				}

				LogError(fmt.Sprintf("Android response failure: %v type=%v", resp, me.AndroidPushSettings.Type))
				incrementFailure(PUSH_NOTIFY_ANDROID, pushType, fcmError.Error())
				return NewErrorPushResponse(fcmError.Error())
			}
		}
	}

	if len(msg.AckId) > 0 {
		incrementSuccessWithAck(PUSH_NOTIFY_ANDROID, pushType)
	} else {
		incrementSuccess(PUSH_NOTIFY_ANDROID, pushType)
	}
	return NewOkPushResponse()
}
