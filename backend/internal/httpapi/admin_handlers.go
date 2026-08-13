package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/xg-management/platform/backend/internal/admin"
)

func listUsers(repository admin.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		items, err := repository.ListUsers(c, principal.OrganizationID)
		if err != nil {
			internalError(c)
			return
		}
		respondData(c, http.StatusOK, items)
	}
}

func createUser(repository admin.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			DisplayName string `json:"display_name"`
			Email       string `json:"email"`
		}
		if !bindJSON(c, &input) || strings.TrimSpace(input.DisplayName) == "" {
			invalidInput(c, "display_name is required")
			return
		}
		principal, _ := currentPrincipal(c)
		item, err := repository.CreateUser(c, principal.OrganizationID, strings.TrimSpace(input.DisplayName), strings.TrimSpace(input.Email))
		if err != nil {
			repositoryError(c, err)
			return
		}
		respondData(c, http.StatusCreated, item)
	}
}

func updateUser(repository admin.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			DisplayName string `json:"display_name"`
			Email       string `json:"email"`
			Status      string `json:"status"`
		}
		if !bindJSON(c, &input) || (input.Status != "active" && input.Status != "disabled") || strings.TrimSpace(input.DisplayName) == "" {
			invalidInput(c, "display_name and a valid status are required")
			return
		}
		principal, _ := currentPrincipal(c)
		item, err := repository.UpdateUser(c, principal.OrganizationID, c.Param("id"), strings.TrimSpace(input.DisplayName), strings.TrimSpace(input.Email), input.Status)
		if err != nil {
			repositoryError(c, err)
			return
		}
		respondData(c, http.StatusOK, item)
	}
}

func setUserRoles(repository admin.Repository, reloader PolicyReloader) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			RoleIDs []string `json:"role_ids"`
		}
		if !bindJSON(c, &input) {
			return
		}
		principal, _ := currentPrincipal(c)
		if err := repository.SetUserRoles(c, principal.OrganizationID, c.Param("id"), input.RoleIDs); err != nil {
			repositoryError(c, err)
			return
		}
		if !reloadPolicy(c, reloader) {
			return
		}
		respondData(c, http.StatusOK, gin.H{"updated": true})
	}
}

func listRoles(repository admin.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		items, err := repository.ListRoles(c, principal.OrganizationID)
		if err != nil {
			internalError(c)
			return
		}
		respondData(c, http.StatusOK, items)
	}
}
func createRole(repository admin.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if !bindJSON(c, &input) || strings.TrimSpace(input.Name) == "" {
			invalidInput(c, "name is required")
			return
		}
		principal, _ := currentPrincipal(c)
		item, err := repository.CreateRole(c, principal.OrganizationID, strings.TrimSpace(input.Name), strings.TrimSpace(input.Description))
		if err != nil {
			repositoryError(c, err)
			return
		}
		respondData(c, http.StatusCreated, item)
	}
}
func updateRole(repository admin.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if !bindJSON(c, &input) || strings.TrimSpace(input.Name) == "" {
			invalidInput(c, "name is required")
			return
		}
		principal, _ := currentPrincipal(c)
		item, err := repository.UpdateRole(c, principal.OrganizationID, c.Param("id"), strings.TrimSpace(input.Name), strings.TrimSpace(input.Description))
		if err != nil {
			repositoryError(c, err)
			return
		}
		respondData(c, http.StatusOK, item)
	}
}
func deleteRole(repository admin.Repository, reloader PolicyReloader) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		if err := repository.DeleteRole(c, principal.OrganizationID, c.Param("id")); err != nil {
			repositoryError(c, err)
			return
		}
		if !reloadPolicy(c, reloader) {
			return
		}
		c.Status(http.StatusNoContent)
	}
}
func setRolePermissions(repository admin.Repository, reloader PolicyReloader) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			Permissions []string `json:"permissions"`
		}
		if !bindJSON(c, &input) {
			return
		}
		principal, _ := currentPrincipal(c)
		if err := repository.SetRolePermissions(c, principal.OrganizationID, c.Param("id"), input.Permissions); err != nil {
			repositoryError(c, err)
			return
		}
		if !reloadPolicy(c, reloader) {
			return
		}
		respondData(c, http.StatusOK, gin.H{"updated": true})
	}
}
func setRoleMenus(repository admin.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input struct {
			MenuIDs []string `json:"menu_ids"`
		}
		if !bindJSON(c, &input) {
			return
		}
		principal, _ := currentPrincipal(c)
		if err := repository.SetRoleMenus(c, principal.OrganizationID, c.Param("id"), input.MenuIDs); err != nil {
			repositoryError(c, err)
			return
		}
		respondData(c, http.StatusOK, gin.H{"updated": true})
	}
}
func listPermissions(repository admin.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		items, err := repository.ListPermissions(c)
		if err != nil {
			internalError(c)
			return
		}
		respondData(c, http.StatusOK, items)
	}
}

func listMenus(repository admin.Repository, own bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		var items []admin.Menu
		var err error
		if own {
			items, err = repository.ListUserMenus(c, principal.OrganizationID, principal.UserID)
			if err == nil {
				permissions := map[string]bool{}
				for _, permission := range principal.Permissions {
					permissions[permission] = true
				}
				filtered := make([]admin.Menu, 0, len(items))
				for _, item := range items {
					if item.RequiredPermission == "" || permissions["*"] || permissions[item.RequiredPermission] {
						filtered = append(filtered, item)
					}
				}
				items = filtered
			}
		} else {
			items, err = repository.ListMenus(c, principal.OrganizationID)
		}
		if err != nil {
			internalError(c)
			return
		}
		respondData(c, http.StatusOK, items)
	}
}
func createMenu(repository admin.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input admin.Menu
		if !bindJSON(c, &input) || !validMenu(input) {
			invalidInput(c, "code, name, and a valid status are required")
			return
		}
		principal, _ := currentPrincipal(c)
		item, err := repository.CreateMenu(c, principal.OrganizationID, input)
		if err != nil {
			repositoryError(c, err)
			return
		}
		respondData(c, http.StatusCreated, item)
	}
}
func updateMenu(repository admin.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input admin.Menu
		if !bindJSON(c, &input) || !validMenu(input) {
			invalidInput(c, "code, name, and a valid status are required")
			return
		}
		input.ID = c.Param("id")
		principal, _ := currentPrincipal(c)
		item, err := repository.UpdateMenu(c, principal.OrganizationID, input)
		if err != nil {
			repositoryError(c, err)
			return
		}
		respondData(c, http.StatusOK, item)
	}
}
func deleteMenu(repository admin.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		if err := repository.DeleteMenu(c, principal.OrganizationID, c.Param("id")); err != nil {
			repositoryError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func listSettings(repository admin.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		items, err := repository.ListSettings(c, principal.OrganizationID)
		if err != nil {
			internalError(c)
			return
		}
		respondData(c, http.StatusOK, items)
	}
}
func upsertSetting(repository admin.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var input admin.Setting
		if !bindJSON(c, &input) || strings.TrimSpace(input.Namespace) == "" || strings.TrimSpace(input.Key) == "" || len(input.Value) == 0 || !json.Valid(input.Value) {
			invalidInput(c, "namespace, key and valid JSON value are required")
			return
		}
		principal, _ := currentPrincipal(c)
		item, err := repository.UpsertSetting(c, principal.OrganizationID, principal.UserID, input)
		if err != nil {
			repositoryError(c, err)
			return
		}
		respondData(c, http.StatusOK, item)
	}
}
func deleteSetting(repository admin.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		principal, _ := currentPrincipal(c)
		if err := repository.DeleteSetting(c, principal.OrganizationID, c.Param("namespace"), c.Param("key")); err != nil {
			repositoryError(c, err)
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func bindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		invalidInput(c, "request body must be valid JSON")
		return false
	}
	return true
}
func validMenu(item admin.Menu) bool {
	return strings.TrimSpace(item.Code) != "" && strings.TrimSpace(item.Name) != "" && (item.Status == "active" || item.Status == "hidden")
}

func reloadPolicy(c *gin.Context, reloader PolicyReloader) bool {
	if reloader == nil || reloader.Reload(c.Request.Context()) != nil {
		internalError(c)
		return false
	}
	return true
}
func invalidInput(c *gin.Context, message string) {
	respondError(c, http.StatusBadRequest, "invalid_input", message)
}
func internalError(c *gin.Context) {
	respondError(c, http.StatusInternalServerError, "internal_error", "The request could not be completed.")
}
func repositoryError(c *gin.Context, _ error) {
	respondError(c, http.StatusBadRequest, "repository_error", "The requested change was rejected.")
}
