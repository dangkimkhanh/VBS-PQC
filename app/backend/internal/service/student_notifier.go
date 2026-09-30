package service

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/internal/repository"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// studentRecipients returns where to email a student: the personal email of an
// activated account first (graduates often lose their school mailbox), then the
// school email.
func studentRecipients(ctx context.Context, users repository.UserRepository, accounts repository.AuthRepository, userID primitive.ObjectID) (string, []string) {
	student, err := users.GetUserByID(ctx, userID)
	if err != nil || student == nil {
		return "", nil
	}
	recipients := []string{}
	if account, err := accounts.FindPersonalAccountByUserID(ctx, student.ID); err == nil && account != nil &&
		account.Status == models.AccountActive && strings.TrimSpace(account.PersonalEmail) != "" {
		recipients = append(recipients, account.PersonalEmail)
	}
	if email := utils.NormalizeEmail(student.Email); email != "" && (len(recipients) == 0 || recipients[0] != email) {
		recipients = append(recipients, email)
	}
	return student.FullName, recipients
}

// StudentNotifier emails students about changes to their diplomas.
type StudentNotifier struct {
	users    repository.UserRepository
	accounts repository.AuthRepository
	mail     utils.EmailSender
}

func NewStudentNotifier(users repository.UserRepository, accounts repository.AuthRepository, mail utils.EmailSender) *StudentNotifier {
	return &StudentNotifier{users: users, accounts: accounts, mail: mail}
}

// NotifyRevoked tells each student that the diploma was revoked and why. Sending
// happens in the background; a failed email never undoes the revocation.
func (n *StudentNotifier) NotifyRevoked(ctx context.Context, diplomas []*models.EDiploma, reason string, revokedAt time.Time) {
	if n == nil || n.mail == nil || n.users == nil || n.accounts == nil {
		return
	}
	type message struct{ to, body string }
	messages := []message{}
	for _, d := range diplomas {
		if d == nil || d.UserID.IsZero() {
			continue
		}
		name, recipients := studentRecipients(ctx, n.users, n.accounts, d.UserID)
		if len(recipients) == 0 {
			continue
		}
		if name == "" {
			name = d.FullName
		}
		body := fmt.Sprintf("Xin chào %s,\n\nVăn bằng số của bạn đã bị cơ sở đào tạo thu hồi và không còn hiệu lực.\n\nMã sinh viên: %s\nTên văn bằng: %s\nSố hiệu: %s\nThời điểm thu hồi: %s\nLý do thu hồi: %s\n\nTrang xác minh công khai sẽ hiển thị văn bằng này là đã thu hồi: %s\nNếu có thắc mắc, vui lòng liên hệ phòng đào tạo của trường.\n\nEmail này chỉ mang tính thông báo; hệ thống không bao giờ gửi mật khẩu hay khóa bí mật qua email.",
			name, d.StudentCode, d.Name, d.SerialNumber, revokedAt.In(vietnamLocation()).Format("15:04 02/01/2006"),
			strings.TrimSpace(reason), fmt.Sprintf("%s/verify/%s", appURL(), d.ID.Hex()))
		for _, to := range recipients {
			messages = append(messages, message{to, body})
		}
	}
	if len(messages) == 0 {
		return
	}
	go func() {
		for _, m := range messages {
			if err := n.mail.SendEmail(m.to, "Thông báo thu hồi văn bằng số - VBS PQC", m.body); err != nil {
				log.Printf("failed to send revocation notice to %s: %v", m.to, err)
			}
		}
	}()
}
