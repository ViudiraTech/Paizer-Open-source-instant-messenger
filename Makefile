# =====================================================
#
#      Makefile
#      Paizer compile script
#
#      Based on MIT open source agreement
#      Copyright © 2020 ViudiraTech, based on the MIT agreement.
#
# =====================================================

GO_SOURCES := $(shell find . -type f -name "*.go" ! -path "./vendor/*")
GO_FLAGS   := build -o
GOLANG     := go

OUTS       := $(patsubst %.go,%.out,$(notdir $(GO_SOURCES)))

.PHONY: all clean

all: $(OUTS)

%.out:
	$(GOLANG) $(GO_FLAGS) $@ $(filter %/$*.go,$(GO_SOURCES))

clean:
	rm -f $(OUTS)
