from __future__ import annotations

import logging
from typing import Any, Dict, Optional, List

from vearch.utils import IndexType, MetricType

logger = logging.getLogger("vearch")


# class IndexParams(NamedTuple):
#     metric_type: str = MetricType.Inner_product
#     training_threshold: int = 100000
#     ncentroids: int = 2048
#     nsubvector: int = 64
#     bucket_init_size: int = 1000
#     buckert_max_size: int = 1280000
#     nlinks: int = 32
#     efConstruction: int = 40


class Index:
    def __init__(
        self, index_name: str, index_type: str, params: Optional[Dict[str, Any]] = None,
        field_name: Optional[str] = None, field_names: Optional[List[str]] = None
    ):
        self._index_name = index_name
        self._index_type = index_type
        self._params = params
        self._field_name = field_name
        self._field_names = field_names

    def to_dict(self):
        d: Dict[str, Any] = {
            "name": self._index_name,
            "type": self._index_type,
        }
        if self._params is not None:
            d["params"] = self._params
        if self._field_name is not None:
            d["field_name"] = self._field_name
        if self._field_names is not None:
            d["field_names"] = self._field_names
        return d

    def dict(self):
        return self.to_dict()

    @classmethod
    def from_dict(cls, index_data: Dict) -> Index:
        return cls(
            index_data["name"],
            index_data["type"],
            index_data.get("params", None),
            index_data.get("field_name", None),
            index_data.get("field_names", None)
        )


class ScalarIndex(Index):
    def __init__(self, index_name: str, field_name: str = None):
        super().__init__(index_name, IndexType.SCALAR, field_name=field_name)


class InvertedIndex(Index):
    def __init__(self, index_name: str, field_name: str = None):
        super().__init__(index_name, IndexType.INVERTED, field_name=field_name)


class BitmapIndex(Index):
    def __init__(self, index_name: str, field_name: str = None):
        super().__init__(index_name, IndexType.BITMAP, field_name=field_name)


class CompositeIndex(Index):
    def __init__(self, index_name: str, field_names: List[str]):
        super().__init__(index_name, IndexType.COMPOSITE, field_names=field_names)


class IvfPQIndex(Index):
    def __init__(
        self,
        index_name: str,
        metric_type: str,
        ncentroids: int,
        nsubvector: int,
        training_threshold: Optional[int] = None,
        bucket_init_size: int = 1000,
        bucket_max_size: int = 1280000,
        nprobe: int = 80,
        hnsw: Optional[HNSWIndex] = None,
        field_name: Optional[str] = None
    ):
        params = {
            "metric_type": metric_type,
            "ncentroids": ncentroids,
            "nsubvector": nsubvector,
            "bucket_init_size": bucket_init_size,
            "bucket_max_size": bucket_max_size,
            "training_threshold": training_threshold
            if training_threshold
            else int(ncentroids * 200),
            "nprobe": nprobe
        }
        if hnsw is not None:
            params["hnsw"] = {}
            params["hnsw"]["nlinks"] = hnsw._params.get("nlinks", None)
            params["hnsw"]["efConstruction"] = hnsw._params.get("efConstruction", None)
            params["hnsw"]["efSearch"] = hnsw._params.get("efSearch", None)
        super().__init__(index_name, IndexType.IVFPQ, params, field_name)

    def nsubvector(self):
        if self._params is None:
            return None
        return self._params["nsubvector"]


class IvfFlatIndex(Index):
    def __init__(
        self,
        index_name: str,
        metric_type: str,
        ncentroids: int,
        training_threshold: Optional[int] = None,
        nprobe: int = 80,
        hnsw: Optional[HNSWIndex] = None,
        field_name: Optional[str] = None
    ):
        params = {
            "metric_type": metric_type,
            "ncentroids": ncentroids,
            "training_threshold": training_threshold
            if training_threshold
            else int(ncentroids * 200),
            "nprobe": nprobe
        }
        if hnsw is not None:
            params["hnsw"] = {}
            params["hnsw"]["nlinks"] = hnsw._params.get("nlinks", None)
            params["hnsw"]["efConstruction"] = hnsw._params.get("efConstruction", None)
            params["hnsw"]["efSearch"] = hnsw._params.get("efSearch", None)
        super().__init__(index_name, IndexType.IVFFLAT, params, field_name)


class BinaryIvfIndex(Index):
    """
    check vector length is powder of 8
    """

    def __init__(self, index_name: str, ncentroids: int, field_name: Optional[str] = None):
        params = {
            "ncentroids": ncentroids,
        }
        super().__init__(index_name, IndexType.BINARYIVF, params, field_name)


class FlatIndex(Index):
    def __init__(self, index_name: str, metric_type: str, field_name: Optional[str] = None):
        super().__init__(index_name, IndexType.FLAT, {"metric_type": metric_type}, field_name)


class HNSWIndex(Index):
    def __init__(
        self,
        index_name: str = None,
        metric_type: str = None,
        nlinks: int = 32,
        efConstruction: int = 160,
        efSearch: int = 64,
        field_name: Optional[str] = None
    ):
        params = dict(
            metric_type=metric_type, nlinks=nlinks, efConstruction=efConstruction, efSearch=efSearch
        )
        super().__init__(index_name, IndexType.HNSW, params)


class GPUIvfPQIndex(Index):
    def __init__(
        self,
        index_name: str,
        metric_type: str,
        ncentroids: int,
        nsubvector: int,
        training_threshold: Optional[int] = None,
        nprobe: int = 80,
        field_name: Optional[str] = None
    ):
        params = dict(
            metric_type=metric_type,
            ncentroids=ncentroids,
            nsubvector=nsubvector,
            nprobe=nprobe,
            training_threshold=training_threshold
            if training_threshold
            else int(ncentroids * 200),
        )
        super().__init__(index_name, IndexType.GPU_IVFPQ, params, field_name)


class GPUIvfFlatIndex(Index):
    def __init__(
        self,
        index_name: str,
        metric_type: str,
        ncentroids: int,
        training_threshold: Optional[int] = None,
        bucket_init_size: int = 1000,
        bucket_max_size: int = 1280000,
        nprobe: int = 80,
        field_name: Optional[str] = None
    ):
        params = {
            "metric_type": metric_type,
            "ncentroids": ncentroids,
            "bucket_init_size": bucket_init_size,
            "bucket_max_size": bucket_max_size,
            "training_threshold": training_threshold
            if training_threshold
            else int(ncentroids * 200),
            "nprobe": nprobe
        }
        super().__init__(index_name, IndexType.GPU_IVFFLAT, params, field_name)


class NPUIvfRaBitQIndex(Index):
    def __init__(
        self,
        index_name: str,
        metric_type: str = MetricType.L2,
        ncentroids: int = 4096,
        nb_bits: int = 1,
        training_threshold: Optional[int] = None,
        nprobe: int = 80,
        field_name: Optional[str] = None
    ):
        params = {
            "metric_type": metric_type,
            "ncentroids": ncentroids,
            "nb_bits": nb_bits,
            "training_threshold": training_threshold
            if training_threshold
            else int(ncentroids * 200),
            "nprobe": nprobe
        }
        super().__init__(index_name, IndexType.NPU_IVFRABITQ, params, field_name)

    def nb_bits(self):
        if self._params is None:
            return None
        return self._params["nb_bits"]


class IvfRaBitQIndex(Index):
    def __init__(
        self,
        index_name: str,
        metric_type: str,
        ncentroids: int,
        nb_bits: int,
        training_threshold: Optional[int] = None,
        bucket_init_size: int = 1000,
        bucket_max_size: int = 1280000,
        nprobe: int = 80,
        hnsw: Optional[HNSWIndex] = None,
        field_name: Optional[str] = None
    ):
        params = {
            "metric_type": metric_type,
            "ncentroids": ncentroids,
            "nb_bits": nb_bits,
            "bucket_init_size": bucket_init_size,
            "bucket_max_size": bucket_max_size,
            "training_threshold": training_threshold
            if training_threshold
            else int(ncentroids * 200),
            "nprobe": nprobe
        }
        if hnsw is not None:
            params["hnsw"] = {}
            params["hnsw"]["nlinks"] = hnsw._params.get("nlinks", None)
            params["hnsw"]["efConstruction"] = hnsw._params.get("efConstruction", None)
            params["hnsw"]["efSearch"] = hnsw._params.get("efSearch", None)
        super().__init__(index_name, IndexType.IVFRABITQ, params, field_name)

    def nb_bits(self):
        if self._params is None:
            return None
        return self._params["nb_bits"]