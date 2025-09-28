# User Management Commands

This application now includes command-line user management functions that can be used while the server is running.

## Available Commands

- `makeAdmin <email>` - Grant admin privileges to a user
- `removeAdmin <email>` - Remove admin privileges from a user
- `activateUser <email>` - Activate/verify a user account
- `deleteUser <email>` - Delete a user account and all associated data
- `help` - Show available commands
- `exit` or `quit` - Exit the server

## Usage

1. Start the server: `go run main.go`
2. The CLI will automatically start and show a prompt: `hcs>`
3. Type commands followed by email addresses

## Examples

```
hcs> help
hcs> makeAdmin user@example.com
hcs> activateUser newuser@example.com
hcs> removeAdmin oldadmin@example.com
hcs> deleteUser spammer@example.com
hcs> exit
```

## Features

- **Input validation**: Commands check if users exist before performing operations
- **Status checking**: Commands won't duplicate actions (e.g., won't make an already-admin user admin again)
- **Cascade deletion**: Deleting a user removes all their appointments and cars
- **Real-time operation**: Commands work while the web server is running
- **Error handling**: Clear error messages for invalid commands or missing users

## Database Operations

The commands use the following SQL operations:
- `MakeUserAdmin`: Updates `is_admin = TRUE`
- `RemoveUserAdmin`: Updates `is_admin = FALSE`
- `ActivateUserByEmail`: Updates `email_verified = TRUE`
- `DeleteUserByEmail`: Deletes user and cascades to related data