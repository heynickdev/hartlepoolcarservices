package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// ShowHelp displays available command-line commands
func ShowHelp() {
	fmt.Println("\n=== User Management Commands ===")
	fmt.Println("Available commands while server is running:")
	fmt.Println("")
	fmt.Println("makeAdmin <email>     - Grant admin privileges to user")
	fmt.Println("removeAdmin <email>   - Remove admin privileges from user")
	fmt.Println("activateUser <email>  - Activate/verify user account")
	fmt.Println("deleteUser <email>    - Delete user account and all associated data")
	fmt.Println("help                  - Show this help message")
	fmt.Println("")
	fmt.Println("Usage: Type the command followed by the email address")
	fmt.Println("Example: makeAdmin user@example.com")
	fmt.Println("")
}

// MakeAdmin grants admin privileges to a user by email
func MakeAdmin(email string) error {
	if email == "" {
		return fmt.Errorf("email address is required")
	}

	// Check if user exists
	user, err := Queries.GetUserByEmail(context.Background(), email)
	if err != nil {
		return fmt.Errorf("user with email '%s' not found", email)
	}

	// Check if already admin
	if user.IsAdmin {
		fmt.Printf("User '%s' is already an admin\n", email)
		return nil
	}

	// Make user admin
	err = Queries.MakeUserAdmin(context.Background(), email)
	if err != nil {
		return fmt.Errorf("failed to make user admin: %v", err)
	}

	// Invalidate all user sessions to force re-login with new privileges
	userID, err := uuid.FromBytes(user.ID.Bytes[:])
	if err == nil {
		BlacklistAllUserTokens(userID, "admin_granted")
	}

	fmt.Printf("Successfully granted admin privileges to '%s' (user logged out)\n", email)
	return nil
}

// RemoveAdmin removes admin privileges from a user by email
func RemoveAdmin(email string) error {
	if email == "" {
		return fmt.Errorf("email address is required")
	}

	// Check if user exists
	user, err := Queries.GetUserByEmail(context.Background(), email)
	if err != nil {
		return fmt.Errorf("user with email '%s' not found", email)
	}

	// Check if not admin
	if !user.IsAdmin {
		fmt.Printf("User '%s' is not an admin\n", email)
		return nil
	}

	// Remove admin privileges
	err = Queries.RemoveUserAdmin(context.Background(), email)
	if err != nil {
		return fmt.Errorf("failed to remove admin privileges: %v", err)
	}

	// Invalidate all user sessions to force re-login with updated privileges
	userID, err := uuid.FromBytes(user.ID.Bytes[:])
	if err == nil {
		BlacklistAllUserTokens(userID, "admin_removed")
	}

	fmt.Printf("Successfully removed admin privileges from '%s' (user logged out)\n", email)
	return nil
}

// ActivateUser activates/verifies a user account by email
func ActivateUser(email string) error {
	if email == "" {
		return fmt.Errorf("email address is required")
	}

	// Check if user exists
	user, err := Queries.GetUserByEmail(context.Background(), email)
	if err != nil {
		return fmt.Errorf("user with email '%s' not found", email)
	}

	// Check if already activated
	if user.EmailVerified {
		fmt.Printf("User '%s' is already activated\n", email)
		return nil
	}

	// Activate user
	err = Queries.ActivateUserByEmail(context.Background(), email)
	if err != nil {
		return fmt.Errorf("failed to activate user: %v", err)
	}

	fmt.Printf("Successfully activated user '%s'\n", email)
	return nil
}

// DeleteUser deletes a user account and all associated data by email
func DeleteUser(email string) error {
	if email == "" {
		return fmt.Errorf("email address is required")
	}

	// Check if user exists
	user, err := Queries.GetUserByEmail(context.Background(), email)
	if err != nil {
		return fmt.Errorf("user with email '%s' not found", email)
	}

	// Invalidate all user sessions before deletion
	userID, err := uuid.FromBytes(user.ID.Bytes[:])
	if err == nil {
		BlacklistAllUserTokens(userID, "user_deleted")
	}

	// Delete user (this will cascade delete appointments and cars due to foreign key constraints)
	err = Queries.DeleteUserByEmail(context.Background(), email)
	if err != nil {
		return fmt.Errorf("failed to delete user: %v", err)
	}

	fmt.Printf("Successfully deleted user '%s' and all associated data (user logged out)\n", email)
	return nil
}

// ProcessCommand processes a command input and executes the appropriate function
func ProcessCommand(input string) error {
	input = strings.TrimSpace(input)
	if input == "" {
		return fmt.Errorf("no command provided")
	}

	parts := strings.Fields(input)
	command := strings.ToLower(parts[0])

	switch command {
	case "help":
		ShowHelp()
		return nil
	case "makeadmin":
		if len(parts) < 2 {
			return fmt.Errorf("makeAdmin requires an email address")
		}
		return MakeAdmin(parts[1])
	case "removeadmin":
		if len(parts) < 2 {
			return fmt.Errorf("removeAdmin requires an email address")
		}
		return RemoveAdmin(parts[1])
	case "activateuser":
		if len(parts) < 2 {
			return fmt.Errorf("activateUser requires an email address")
		}
		return ActivateUser(parts[1])
	case "deleteuser":
		if len(parts) < 2 {
			return fmt.Errorf("deleteUser requires an email address")
		}
		return DeleteUser(parts[1])
	default:
		return fmt.Errorf("unknown command '%s'. Type 'help' for available commands", command)
	}
}
