package servicemanager

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/qiniu/zeroops/internal/config"
	"github.com/qiniu/zeroops/internal/service_manager/api"
	"github.com/qiniu/zeroops/internal/service_manager/database"
	"github.com/qiniu/zeroops/internal/service_manager/service"
	"github.com/rs/zerolog/log"
)

type ServiceManagerServer struct {
	config  *config.Config
	db      *database.Database
	service *service.Service
}

func NewServiceManagerServer(cfg *config.Config) (*ServiceManagerServer, error) {
	db, err := database.NewDatabase(&cfg.Database)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	svc := service.NewService(db)

	server := &ServiceManagerServer{
		config:  cfg,
		db:      db,
		service: svc,
	}

	log.Info().Msg("api initialized successfully with database and service")
	return server, nil
}

func (s *ServiceManagerServer) UseApi(router *gin.Engine) error {
	_, err := api.NewApi(s.db, s.service, router)
	if err != nil {
		return err
	}
	return nil
}

func (s *ServiceManagerServer) Close() error {
	if s.service != nil {
		s.service.Close()
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}
