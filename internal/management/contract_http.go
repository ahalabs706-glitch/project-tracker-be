package management

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"projectflow-backend/internal/auth"
)

func registerContractRoutes(router *gin.RouterGroup, authentication *auth.Service, api *API) {
	router.GET("/contracts", authentication.RequirePermission("contracts.read"), api.listContracts)
	router.GET("/contracts/expiring", authentication.RequirePermission("contracts.read"), api.listExpiringContracts)
	router.POST("/contracts", authentication.RequirePermission("contracts.create"), api.createContract)
	router.GET("/contracts/:id", authentication.RequirePermission("contracts.read"), api.getContract)
	router.PUT("/contracts/:id", authentication.RequirePermission("contracts.update"), api.updateContract)
	router.GET("/contracts/:id/renewals", authentication.RequirePermission("contracts.read"), api.listContractRenewals)
	router.POST("/contracts/:id/renewals", authentication.RequirePermission("contracts.renew"), api.renewContract)
}

func (a *API) listContracts(c *gin.Context) {
	filter, err := parseContractFilter(c, false)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	contracts, meta, err := a.service.ListContracts(c.Request.Context(), principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Contracts retrieved successfully", Data: contracts, Meta: meta})
}

func (a *API) listExpiringContracts(c *gin.Context) {
	filter, err := parseContractFilter(c, true)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	contracts, meta, err := a.service.ExpiringContracts(c.Request.Context(), principal, filter)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Expiring contracts retrieved", Data: contracts, Meta: meta})
}

func (a *API) createContract(c *gin.Context) {
	var input ContractInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	contract, err := a.service.CreateContract(c.Request.Context(), input, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusCreated, successResponse{Success: true, Message: "Contract created successfully", Data: contract})
}

func (a *API) getContract(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	contract, err := a.service.Contract(c.Request.Context(), id, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Contract retrieved successfully", Data: contract})
}

func (a *API) updateContract(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input ContractInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	contract, err := a.service.UpdateContract(c.Request.Context(), id, input, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Contract updated successfully", Data: contract})
}

func (a *API) listContractRenewals(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	renewals, err := a.service.ContractRenewals(c.Request.Context(), id, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusOK, successResponse{Success: true, Message: "Contract renewals retrieved successfully", Data: renewals})
}

func (a *API) renewContract(c *gin.Context) {
	id, ok := resourceID(c)
	if !ok {
		writeManagementError(c, fmt.Errorf("%w: id must be a UUID", ErrInvalid), a.log)
		return
	}
	var input ContractRenewalInput
	if err := c.ShouldBindJSON(&input); err != nil {
		writeManagementError(c, fmt.Errorf("%w: invalid JSON body", ErrInvalid), a.log)
		return
	}
	principal, exists := auth.CurrentPrincipal(c)
	if !exists {
		writeManagementError(c, auth.ErrUnauthorized, a.log)
		return
	}
	contract, err := a.service.RenewContract(c.Request.Context(), id, input, principal)
	if err != nil {
		writeManagementError(c, err, a.log)
		return
	}
	c.JSON(http.StatusCreated, successResponse{Success: true, Message: "Contract renewed successfully", Data: contract})
}

func parseContractFilter(c *gin.Context, expiring bool) (ContractFilter, error) {
	page, err := queryInt(c, "page", 1)
	if err != nil || page < 1 {
		return ContractFilter{}, fmt.Errorf("%w: page must be a positive integer", ErrInvalid)
	}
	perPage, err := queryInt(c, "per_page", 20)
	if err != nil || perPage < 1 || perPage > 100 {
		return ContractFilter{}, fmt.Errorf("%w: per_page must be between 1 and 100", ErrInvalid)
	}
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 100 {
		return ContractFilter{}, fmt.Errorf("%w: search must be at most 100 characters", ErrInvalid)
	}
	filter := ContractFilter{
		Page: page, PerPage: perPage, Search: search,
		Status: c.Query("status"), DepartmentID: c.Query("department_id"),
		OwnerID: c.Query("owner_id"), ProjectID: c.Query("project_id"),
		ExpirationFrom: c.Query("expiration_from"), ExpirationTo: c.Query("expiration_to"),
		SortBy:    c.DefaultQuery("sort_by", "end_date"),
		SortOrder: c.DefaultQuery("sort_order", "asc"),
	}
	if expiring {
		filter.ExpiringWithin, err = parseExpiringWithin(c.Query("within_days"))
		if err != nil {
			return ContractFilter{}, err
		}
		filter.ExpiringOnly = true
	}
	return filter, nil
}
