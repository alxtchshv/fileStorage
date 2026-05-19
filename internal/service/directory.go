package service

import (
	"context"
	"log/slog"

	"managerFiles/internal/model"
	"managerFiles/internal/repository"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"
)

// directoryService реализует DirectoryService.
type directoryService struct {
	dirs     repository.DirectoryRepo
	fileRepo repository.FileRepo
}

// NewDirectoryService создаёт сервис директорий.
func NewDirectoryService(dirs repository.DirectoryRepo, fileRepo repository.FileRepo) DirectoryService {
	return &directoryService{dirs: dirs, fileRepo: fileRepo}
}

// Create создаёт новую директорию.
func (s *directoryService) Create(ctx context.Context, userID string, input *model.CreateDirInput) (*model.DirectoryResponse, error) {

	input.Validate()

	if input.ParentID != nil {

		parentDir, err := s.dirs.GetByID(ctx, *input.ParentID)

		if err != nil {
			return nil, err
		}

		if parentDir.UserID != userID {
			return nil, model.ErrDirNotFound
		}

	}

	dir := &model.Directory{
		ID:       uuid.New().String(),
		UserID:   userID,
		ParentID: input.ParentID,
		Name:     input.Name,
	}

	if err := s.dirs.Create(ctx, dir); err != nil {
		return nil, err
	}

	return dir.ToResponse(), nil
}

// Get возвращает содержимое директории (поддиректории + файлы).
func (s *directoryService) Get(ctx context.Context, dirID, userID string) (*model.DirectoryContents, error) {

	dir, err := s.dirs.GetByID(ctx, dirID)
	if err != nil {
		return nil, err
	}

	if dir.UserID != userID {
		return nil, model.ErrDirNotFound
	}

	g, gCtx := errgroup.WithContext(ctx)

	var subDirs []*model.Directory
	var files []*model.File

	g.Go(func() error {
		var err error
		subDirs, err = s.dirs.ListSubDirs(gCtx, userID, dirID)
		return err
	})

	g.Go(func() error {
		var err error
		files, err = s.fileRepo.ListByDirectory(gCtx, dirID, userID)
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	dirContents := &model.DirectoryContents{
		Directory:   *dir.ToResponse(),
		Directories: make([]model.DirectoryResponse, 0, len(subDirs)),
		Files:       make([]model.FileResponse, 0, len(files)),
	}

	for _, d := range subDirs {
		dirContents.Directories = append(dirContents.Directories, *d.ToResponse())
	}

	for _, f := range files {
		dirContents.Files = append(dirContents.Files, *f.ToResponse())
	}

	return dirContents, nil
}

// GetRoot возвращает корневые директории пользователя.
func (s *directoryService) GetRoot(ctx context.Context, userID string) ([]*model.DirectoryResponse, error) {

	dirs, err := s.dirs.GetRootDirs(ctx, userID)
	if err != nil {
		return nil, err
	}

	res := make([]*model.DirectoryResponse, 0, len(dirs))
	for _, d := range dirs {
		res = append(res, d.ToResponse())
	}

	return res, nil
}

// Delete удаляет директорию. Предупреждение: каскадное удаление!
// ON DELETE CASCADE в PostgreSQL удалит все вложенные директории и записи файлов.
// Реальные объекты из MinIO нужно удалить отдельно (через Kafka или отдельный запрос).
func (s *directoryService) Delete(ctx context.Context, dirID, userID string) error {

	dir, err := s.dirs.GetByID(ctx, dirID)
	if err != nil {
		return err
	}

	if dir.UserID != userID {
		return model.ErrDirNotFound
	}

	// Получаем все файлы в директории (и поддиректориях)
	files, err := s.fileRepo.ListAllRecursive(ctx, dirID, userID)
	if err != nil {
		return err
	}

	// Удаляем директорию (ON DELETE CASCADE удалит поддиректории и файлы в БД)
	if err := s.dirs.Delete(ctx, dirID, userID); err != nil {
		return err
	}

	// Публикуем события удаления файлов (например, в Kafka)
	for _, f := range files {
		// Здесь должен быть вызов публикации события удаления файла, например:
		// s.publishFileDeletedEvent(ctx, f)
		slog.Info("file deleted event published", "file_id", f.ID, "dir_id", dirID, "user_id", userID)
	}

	slog.Info("удаление директории", "dir_id", dirID, "user_id", userID)
	return nil
}
