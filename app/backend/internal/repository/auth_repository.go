package repository

import (
	"context"
	"time"

	"github.com/vnkmasc/Kmasc/app/backend/internal/models"
	"github.com/vnkmasc/Kmasc/app/backend/utils"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type AuthRepository interface {
	EnsureIndexes(ctx context.Context) error

	SaveOTP(ctx context.Context, otp models.OTP) error
	FindOTPByEmail(ctx context.Context, email string) (*models.OTP, error)
	IncrementOTPAttempts(ctx context.Context, email string) error
	DeleteOTP(ctx context.Context, email string) error

	CreateAccount(ctx context.Context, acc *models.Account) error
	FindByID(ctx context.Context, id primitive.ObjectID) (*models.Account, error)
	FindByPersonalEmail(ctx context.Context, email string) (*models.Account, error)
	FindByLoginEmail(ctx context.Context, email string) (*models.Account, error)
	IsLoginEmailTaken(ctx context.Context, email string, exceptID primitive.ObjectID) (bool, error)
	FindByUniversityID(ctx context.Context, universityID primitive.ObjectID) (*models.Account, error)
	FindPersonalAccountByUserID(ctx context.Context, userID primitive.ObjectID) (*models.Account, error)
	FindByRole(ctx context.Context, role string) ([]models.Account, error)
	GetAllAccounts(ctx context.Context, page, pageSize int) ([]*models.Account, int64, error)

	UpdatePassword(ctx context.Context, accountID primitive.ObjectID, newHash string) error
	SetOTPVerificationToken(ctx context.Context, accountID primitive.ObjectID, tokenHash string, expiresAt time.Time) error
	ActivateStudentAccount(ctx context.Context, accountID primitive.ObjectID, personalEmail, passwordHash string, activatedAt time.Time) error
	UpdateStudentEmail(ctx context.Context, studentID primitive.ObjectID, email string) error
	DeleteByStudentID(ctx context.Context, studentID primitive.ObjectID) error

	FindByActivationTokenHash(ctx context.Context, hash string) (*models.Account, error)
	ActivateAccount(ctx context.Context, id primitive.ObjectID, passwordHash string, activatedAt time.Time) error
	SetActivationToken(ctx context.Context, id primitive.ObjectID, tokenHash string, expiresAt time.Time) error
	UpdateUniversityAdminEmail(ctx context.Context, id primitive.ObjectID, email string) error
	SetStatus(ctx context.Context, id primitive.ObjectID, status string) error

	SetPasswordResetToken(ctx context.Context, id primitive.ObjectID, tokenHash string, expiresAt time.Time) error
	FindByPasswordResetTokenHash(ctx context.Context, hash string) (*models.Account, error)
	ResetPassword(ctx context.Context, id primitive.ObjectID, passwordHash string) error
}

type authRepository struct {
	col    *mongo.Collection
	otpCol *mongo.Collection
}

func NewAuthRepository(db *mongo.Database) AuthRepository {
	return &authRepository{col: db.Collection("accounts"), otpCol: db.Collection("otps")}
}

func (r *authRepository) EnsureIndexes(ctx context.Context) error {
	_, err := r.otpCol.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "email", Value: 1}}, Options: options.Index().SetUnique(true)},
		{Keys: bson.D{{Key: "expires_at", Value: 1}}, Options: options.Index().SetExpireAfterSeconds(0)},
	})
	return err
}

func (r *authRepository) findOne(ctx context.Context, filter bson.M) (*models.Account, error) {
	var account models.Account
	err := r.col.FindOne(ctx, filter).Decode(&account)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &account, nil
}

// ===== OTP =====

func (r *authRepository) SaveOTP(ctx context.Context, otp models.OTP) error {
	otp.Email = utils.NormalizeEmail(otp.Email)
	_, err := r.otpCol.ReplaceOne(ctx, bson.M{"email": otp.Email}, otp, options.Replace().SetUpsert(true))
	return err
}

func (r *authRepository) FindOTPByEmail(ctx context.Context, email string) (*models.OTP, error) {
	var otp models.OTP
	err := r.otpCol.FindOne(ctx, bson.M{"email": utils.NormalizeEmail(email)}).Decode(&otp)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &otp, nil
}

func (r *authRepository) IncrementOTPAttempts(ctx context.Context, email string) error {
	_, err := r.otpCol.UpdateOne(ctx, bson.M{"email": utils.NormalizeEmail(email)}, bson.M{"$inc": bson.M{"attempts": 1}})
	return err
}

func (r *authRepository) DeleteOTP(ctx context.Context, email string) error {
	_, err := r.otpCol.DeleteOne(ctx, bson.M{"email": utils.NormalizeEmail(email)})
	return err
}

// ===== Accounts =====

func (r *authRepository) CreateAccount(ctx context.Context, acc *models.Account) error {
	acc.PersonalEmail = utils.NormalizeEmail(acc.PersonalEmail)
	acc.StudentEmail = utils.NormalizeEmail(acc.StudentEmail)
	_, err := r.col.InsertOne(ctx, acc)
	return err
}

func (r *authRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.Account, error) {
	return r.findOne(ctx, bson.M{"_id": id})
}

func (r *authRepository) FindByPersonalEmail(ctx context.Context, email string) (*models.Account, error) {
	return r.findOne(ctx, bson.M{"personal_email": utils.EmailFilter(email)})
}

// FindByLoginEmail accepts the login email of any role, and also the school
// email of an activated student so a mistyped personal email cannot lock them out.
func (r *authRepository) FindByLoginEmail(ctx context.Context, email string) (*models.Account, error) {
	filter := utils.EmailFilter(email)
	return r.findOne(ctx, bson.M{"$or": bson.A{
		bson.M{"personal_email": filter},
		bson.M{"student_email": filter, "role": "student", "status": models.AccountActive},
	}})
}

func (r *authRepository) IsLoginEmailTaken(ctx context.Context, email string, exceptID primitive.ObjectID) (bool, error) {
	filter := utils.EmailFilter(email)
	count, err := r.col.CountDocuments(ctx, bson.M{
		"_id": bson.M{"$ne": exceptID},
		"$or": bson.A{bson.M{"personal_email": filter}, bson.M{"student_email": filter}},
	})
	return count > 0, err
}

func (r *authRepository) FindByUniversityID(ctx context.Context, universityID primitive.ObjectID) (*models.Account, error) {
	return r.findOne(ctx, bson.M{"university_id": universityID, "role": "university_admin"})
}

func (r *authRepository) FindPersonalAccountByUserID(ctx context.Context, userID primitive.ObjectID) (*models.Account, error) {
	return r.findOne(ctx, bson.M{"student_id": userID, "role": "student"})
}

func (r *authRepository) FindByRole(ctx context.Context, role string) ([]models.Account, error) {
	cursor, err := r.col.Find(ctx, bson.M{"role": role})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var accounts []models.Account
	if err = cursor.All(ctx, &accounts); err != nil {
		return nil, err
	}
	return accounts, nil
}

func (r *authRepository) GetAllAccounts(ctx context.Context, page, pageSize int) ([]*models.Account, int64, error) {
	skip := (page - 1) * pageSize

	total, err := r.col.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}
	opts := options.Find().SetSkip(int64(skip)).SetLimit(int64(pageSize)).SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.col.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var accounts []*models.Account
	if err := cursor.All(ctx, &accounts); err != nil {
		return nil, 0, err
	}

	return accounts, total, nil
}

func (r *authRepository) UpdatePassword(ctx context.Context, accountID primitive.ObjectID, newHash string) error {
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": accountID}, bson.M{"$set": bson.M{
		"password_hash": newHash, "password_changed_at": time.Now(),
	}})
	return err
}

func (r *authRepository) SetOTPVerificationToken(ctx context.Context, accountID primitive.ObjectID, tokenHash string, expiresAt time.Time) error {
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": accountID, "role": "student"}, bson.M{"$set": bson.M{
		"otp_verification_token_hash": tokenHash, "otp_verification_expires_at": expiresAt,
	}})
	return err
}

func (r *authRepository) ActivateStudentAccount(ctx context.Context, accountID primitive.ObjectID, personalEmail, passwordHash string, activatedAt time.Time) error {
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": accountID, "role": "student", "status": bson.M{"$ne": models.AccountLocked}}, bson.M{"$set": bson.M{
		"personal_email": utils.NormalizeEmail(personalEmail), "password_hash": passwordHash, "status": models.AccountActive,
		"activated_at": activatedAt, "password_changed_at": activatedAt,
	}, "$unset": bson.M{"otp_verification_token_hash": "", "otp_verification_expires_at": ""}})
	return err
}

func (r *authRepository) UpdateStudentEmail(ctx context.Context, studentID primitive.ObjectID, email string) error {
	_, err := r.col.UpdateOne(ctx, bson.M{"student_id": studentID, "role": "student"}, bson.M{"$set": bson.M{"student_email": utils.NormalizeEmail(email)}})
	return err
}

func (r *authRepository) DeleteByStudentID(ctx context.Context, studentID primitive.ObjectID) error {
	_, err := r.col.DeleteMany(ctx, bson.M{"student_id": studentID, "role": "student"})
	return err
}

func (r *authRepository) FindByActivationTokenHash(ctx context.Context, hash string) (*models.Account, error) {
	return r.findOne(ctx, bson.M{"activation_token_hash": hash})
}

func (r *authRepository) ActivateAccount(ctx context.Context, id primitive.ObjectID, passwordHash string, activatedAt time.Time) error {
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id, "status": models.AccountPending}, bson.M{"$set": bson.M{
		"password_hash": passwordHash, "status": models.AccountActive, "activated_at": activatedAt, "password_changed_at": activatedAt,
	}, "$unset": bson.M{"activation_token_hash": "", "activation_expires_at": ""}})
	return err
}

func (r *authRepository) SetActivationToken(ctx context.Context, id primitive.ObjectID, tokenHash string, expiresAt time.Time) error {
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id, "role": "university_admin"}, bson.M{"$set": bson.M{
		"activation_token_hash": tokenHash, "activation_expires_at": expiresAt, "status": models.AccountPending,
	}})
	return err
}

// UpdateUniversityAdminEmail moves the admin account to a new email. The old
// password stops working: the new owner must activate the account again.
func (r *authRepository) UpdateUniversityAdminEmail(ctx context.Context, id primitive.ObjectID, email string) error {
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id, "role": "university_admin"}, bson.M{"$set": bson.M{
		"personal_email": utils.NormalizeEmail(email), "password_hash": "", "status": models.AccountPending,
		"password_changed_at": time.Now(),
	}, "$unset": bson.M{"password_reset_token_hash": "", "password_reset_expires_at": ""}})
	return err
}

func (r *authRepository) SetStatus(ctx context.Context, id primitive.ObjectID, status string) error {
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"status": status}})
	return err
}

func (r *authRepository) SetPasswordResetToken(ctx context.Context, id primitive.ObjectID, tokenHash string, expiresAt time.Time) error {
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"password_reset_token_hash": tokenHash, "password_reset_expires_at": expiresAt}})
	return err
}

func (r *authRepository) FindByPasswordResetTokenHash(ctx context.Context, hash string) (*models.Account, error) {
	return r.findOne(ctx, bson.M{"password_reset_token_hash": hash})
}

// ResetPassword never re-opens a locked account.
func (r *authRepository) ResetPassword(ctx context.Context, id primitive.ObjectID, passwordHash string) error {
	now := time.Now()
	_, err := r.col.UpdateOne(ctx, bson.M{"_id": id, "status": bson.M{"$ne": models.AccountLocked}}, bson.M{
		"$set":   bson.M{"password_hash": passwordHash, "status": models.AccountActive, "password_changed_at": now},
		"$unset": bson.M{"password_reset_token_hash": "", "password_reset_expires_at": "", "activation_token_hash": "", "activation_expires_at": ""},
	})
	return err
}
