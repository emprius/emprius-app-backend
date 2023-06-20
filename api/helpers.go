package api

import (
	"github.com/genjidb/genji"
	"github.com/rs/zerolog/log"
)

func closeResult(r *genji.Result) {
	if err := r.Close(); err != nil {
		log.Error().Err(err).Msg("failed to close result")
	}
}
