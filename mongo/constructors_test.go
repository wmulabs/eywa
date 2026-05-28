package mongo

import (
	"testing"

	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

// These tests exercise New*Repository constructors and their ensureIndexes/getIndexModels
// paths. mtest Mock client returns an error for unqueued createIndexes commands; since
// ensureIndexes swallows the error (just logs), the constructor still returns a valid repo.

func TestNewChronicleRepository_ReturnsNonNil(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("ok", func(mt *mtest.T) {
		if repo := NewChronicleRepository(mt.DB); repo == nil {
			t.Fatal("expected non-nil ChronicleRepository")
		}
	})
}

func TestNewEchoRepository_ReturnsNonNil(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("ok", func(mt *mtest.T) {
		if repo := NewEchoRepository(mt.DB); repo == nil {
			t.Fatal("expected non-nil EchoRepository")
		}
	})
}

func TestNewImprintRepository_ReturnsNonNil(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("ok", func(mt *mtest.T) {
		if repo := NewImprintRepository(mt.DB); repo == nil {
			t.Fatal("expected non-nil ImprintRepository")
		}
	})
}

func TestNewLedgerRepository_ReturnsNonNil(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("ok", func(mt *mtest.T) {
		if repo := NewLedgerRepository(mt.DB); repo == nil {
			t.Fatal("expected non-nil LedgerRepository")
		}
	})
}

func TestNewLoreRepository_ReturnsNonNil(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("ok", func(mt *mtest.T) {
		if repo := NewLoreRepository(mt.DB); repo == nil {
			t.Fatal("expected non-nil LoreRepository")
		}
	})
}

func TestNewLoreStore_ReturnsNonNil(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("ok", func(mt *mtest.T) {
		if repo := NewLoreStore(mt.DB); repo == nil {
			t.Fatal("expected non-nil LoreStore")
		}
	})
}

func TestNewOperatorRepository_ReturnsNonNil(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("ok", func(mt *mtest.T) {
		if repo := NewOperatorRepository(mt.DB); repo == nil {
			t.Fatal("expected non-nil OperatorRepository")
		}
	})
}

func TestNewRiteRepository_ReturnsNonNil(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("ok", func(mt *mtest.T) {
		if repo := NewRiteRepository(mt.DB); repo == nil {
			t.Fatal("expected non-nil RiteRepository")
		}
	})
}

func TestNewRitualRepository_ReturnsNonNil(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("ok", func(mt *mtest.T) {
		if repo := NewRitualRepository(mt.DB); repo == nil {
			t.Fatal("expected non-nil RitualRepository")
		}
	})
}

func TestNewSpiritRepository_ReturnsNonNil(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("ok", func(mt *mtest.T) {
		if repo := NewSpiritRepository(mt.DB); repo == nil {
			t.Fatal("expected non-nil SpiritRepository")
		}
	})
}

func TestNewHTTPToolRepository_ReturnsNonNil(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("ok", func(mt *mtest.T) {
		if repo := NewHTTPToolRepository(mt.DB); repo == nil {
			t.Fatal("expected non-nil HTTPToolRepository")
		}
	})
}

func TestNewLinkRepository_ReturnsNonNil(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("ok", func(mt *mtest.T) {
		if repo := NewLinkRepository(mt.DB); repo == nil {
			t.Fatal("expected non-nil LinkRepository")
		}
	})
}

func TestNewWeaveConfigRepository_ReturnsNonNil(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	mt.Run("ok", func(mt *mtest.T) {
		if repo := NewWeaveConfigRepository(mt.DB); repo == nil {
			t.Fatal("expected non-nil WeaveConfigRepository")
		}
	})
}
