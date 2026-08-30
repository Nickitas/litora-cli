package seabed

import (
	"encoding/csv"
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"
)

// Profile задаёт числовой разрез как упорядоченную последовательность узлов
// модели. Автоматический выбор трассы относится к VIEW-03; EXPORT-02 только
// проверяет идентификаторы и сохраняет уже выбранный профиль без интерполяции.
type Profile struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	NodeIDs []int  `json:"node_ids"`
}

// ExportBundleConfig задаёт каталог, пространственные метаданные и готовые
// профили для согласованного набора артефактов EXPORT-02.
type ExportBundleConfig struct {
	Directory string
	Metadata  ExportMetadata
	Profiles  []Profile
}

// ExportBundleArtifacts возвращает точные пути созданных файлов.
type ExportBundleArtifacts struct {
	VTUPath          string `json:"vtu_path"`
	NodesCSVPath     string `json:"nodes_csv_path"`
	CellsCSVPath     string `json:"cells_csv_path"`
	ProfilesCSVPath  string `json:"profiles_csv_path"`
	MetadataJSONPath string `json:"metadata_json_path"`
}

// WriteExportBundle создаёт единый набор black-sea-depth.vtu, nodes.csv,
// cells.csv, profiles.csv и export-metadata.json. Все идентификаторы в таблицах
// совпадают с MSH/VTU, а профили ссылаются только на существующие узлы.
func WriteExportBundle(model Model, config ExportBundleConfig) (ExportBundleArtifacts, error) {
	directory := strings.TrimSpace(config.Directory)
	if directory == "" {
		return ExportBundleArtifacts{}, fmt.Errorf("каталог экспорта не задан")
	}
	if err := validateMSHModel(model); err != nil {
		return ExportBundleArtifacts{}, fmt.Errorf("проверка модели для EXPORT-02: %w", err)
	}
	metadata, err := normalizeExportMetadata(config.Metadata, model)
	if err != nil {
		return ExportBundleArtifacts{}, err
	}
	if err := validateProfiles(model, config.Profiles); err != nil {
		return ExportBundleArtifacts{}, err
	}
	artifacts := ExportBundleArtifacts{
		VTUPath:          filepath.Join(directory, "black-sea-depth.vtu"),
		NodesCSVPath:     filepath.Join(directory, "nodes.csv"),
		CellsCSVPath:     filepath.Join(directory, "cells.csv"),
		ProfilesCSVPath:  filepath.Join(directory, "profiles.csv"),
		MetadataJSONPath: filepath.Join(directory, "export-metadata.json"),
	}
	if err := WriteVTU(artifacts.VTUPath, model, metadata); err != nil {
		return ExportBundleArtifacts{}, err
	}
	if err := WriteNodesCSV(artifacts.NodesCSVPath, model, metadata); err != nil {
		return ExportBundleArtifacts{}, err
	}
	if err := WriteCellsCSV(artifacts.CellsCSVPath, model, metadata); err != nil {
		return ExportBundleArtifacts{}, err
	}
	if err := WriteProfilesCSV(artifacts.ProfilesCSVPath, model, config.Profiles, metadata); err != nil {
		return ExportBundleArtifacts{}, err
	}
	if err := WriteExportMetadataJSON(artifacts.MetadataJSONPath, metadata); err != nil {
		return ExportBundleArtifacts{}, err
	}
	return artifacts, nil
}

// WriteProfilesCSV сохраняет готовые профили с накопленным расстоянием по X/Y
// LAEA и исходными узловыми отметками. PointIndex начинается с 1, DistanceM —
// с нуля; координаты и глубина не интерполируются и остаются проверяемыми по ID.
func WriteProfilesCSV(path string, model Model, profiles []Profile, metadata ExportMetadata) error {
	if err := validateMSHModel(model); err != nil {
		return fmt.Errorf("проверка модели для profiles.csv: %w", err)
	}
	normalized, err := normalizeExportMetadata(metadata, model)
	if err != nil {
		return err
	}
	if err := validateProfiles(model, profiles); err != nil {
		return err
	}
	file, err := createOutputFile(path)
	if err != nil {
		return err
	}
	writer := csv.NewWriter(file)
	header := []string{
		"profile_id", "profile_name", "point_index", "distance_m", "node_id",
		"x_m", "y_m", "longitude_deg", "latitude_deg", "elevation_m",
		"water_depth_m", "sampling_method", "quality_flag", "boundary_kind",
	}
	header = append(header, "record_type")
	header = append(header, exportMetadataCSVHeader()...)
	writeErr := writer.Write(header)
	if writeErr == nil {
		row := make([]string, 14)
		row = append(row, "metadata")
		row = append(row, exportMetadataCSVValues(normalized)...)
		writeErr = writer.Write(row)
	}
	for _, profile := range profiles {
		distanceM := 0.0
		for index, nodeID := range profile.NodeIDs {
			if writeErr != nil {
				break
			}
			node := model.Nodes[nodeID]
			if index > 0 {
				previous := model.Nodes[profile.NodeIDs[index-1]]
				distanceM += math.Hypot(node.XM-previous.XM, node.YM-previous.YM)
			}
			row := []string{
				profile.ID,
				profile.Name,
				strconv.Itoa(index + 1),
				formatFloat(distanceM),
				strconv.Itoa(node.ID),
				formatFloat(node.XM),
				formatFloat(node.YM),
				formatFloat(node.LongitudeDeg),
				formatFloat(node.LatitudeDeg),
				formatOptionalFloat(node.ElevationM),
				formatOptionalFloat(node.WaterDepthM),
				string(node.SamplingMethod),
				string(node.QualityFlag),
				string(node.BoundaryKind),
			}
			row = append(row, "data")
			row = append(row, make([]string, len(exportMetadataCSVHeader()))...)
			writeErr = writer.Write(row)
		}
	}
	writer.Flush()
	if writeErr == nil {
		writeErr = writer.Error()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return fmt.Errorf("запись profiles.csv %q: %w", path, writeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("закрытие profiles.csv %q: %w", path, closeErr)
	}
	return nil
}

func validateProfiles(model Model, profiles []Profile) error {
	if len(profiles) == 0 {
		return fmt.Errorf("для profiles.csv не задан ни один профиль; автоматический выбор трасс относится к VIEW-03")
	}
	seenIDs := make(map[string]bool, len(profiles))
	for _, profile := range profiles {
		id := strings.TrimSpace(profile.ID)
		if id == "" || strings.TrimSpace(profile.Name) == "" {
			return fmt.Errorf("профиль должен иметь непустые id и название")
		}
		if seenIDs[id] {
			return fmt.Errorf("идентификатор профиля %q повторяется", id)
		}
		seenIDs[id] = true
		if len(profile.NodeIDs) < 2 {
			return fmt.Errorf("профиль %q должен содержать не менее двух узлов", id)
		}
		for index, nodeID := range profile.NodeIDs {
			if nodeID <= 0 || nodeID >= len(model.Nodes) || model.Nodes[nodeID].ID != nodeID {
				return fmt.Errorf("профиль %q ссылается на отсутствующий узел %d", id, nodeID)
			}
			if index > 0 && profile.NodeIDs[index-1] == nodeID {
				return fmt.Errorf("профиль %q повторяет узел %d подряд", id, nodeID)
			}
			if !validFiniteNonNegative(*model.Nodes[nodeID].WaterDepthM) {
				return fmt.Errorf("профиль %q содержит узел %d без корректной глубины", id, nodeID)
			}
		}
	}
	return nil
}
