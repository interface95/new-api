package model

// TopUpUserInfo is the account summary shown with an administrator's top-up records.
type TopUpUserInfo struct {
	Username  string `json:"username"`
	Quota     int    `json:"quota"`
	UsedQuota int    `json:"used_quota"`
}

type AdminTopUp struct {
	TopUp
	User *TopUpUserInfo `json:"user"`
}

// GetAdminTopUpItems resolves only the users present on the current page.
func GetAdminTopUpItems(topUps []*TopUp) ([]AdminTopUp, error) {
	items := make([]AdminTopUp, len(topUps))
	if len(topUps) == 0 {
		return items, nil
	}

	userIDSet := make(map[int]struct{}, len(topUps))
	userIDs := make([]int, 0, len(topUps))
	for _, topUp := range topUps {
		if _, exists := userIDSet[topUp.UserId]; !exists {
			userIDSet[topUp.UserId] = struct{}{}
			userIDs = append(userIDs, topUp.UserId)
		}
	}

	var users []User
	// Admin user lists also retain soft-deleted accounts for historical records.
	if err := DB.Unscoped().Select("id", "username", "quota", "used_quota").
		Where("id IN ?", userIDs).Find(&users).Error; err != nil {
		return nil, err
	}
	userInfoByID := make(map[int]*TopUpUserInfo, len(users))
	for _, user := range users {
		userInfoByID[user.Id] = &TopUpUserInfo{
			Username:  user.Username,
			Quota:     user.Quota,
			UsedQuota: user.UsedQuota,
		}
	}
	for i, topUp := range topUps {
		items[i] = AdminTopUp{TopUp: *topUp, User: userInfoByID[topUp.UserId]}
	}
	return items, nil
}
