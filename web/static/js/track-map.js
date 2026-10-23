(() => {
  "use strict";

  function initializeMaps(root) {
    const selector = "[data-track-map]:not([data-track-map-initialized])";

    const maps = [];

    if (root.matches?.(selector)) {
      maps.push(root);
    }

    maps.push(...root.querySelectorAll(selector));

    maps.forEach(initializeMap);
  }

  function initializeMap(element) {
    if (typeof window.L === "undefined") {
      return;
    }

    const rawData = element.dataset.trackMapData;

    if (!rawData) {
      return;
    }

    let data;

    try {
      data = JSON.parse(rawData);
    } catch (error) {
      console.error("Unable to parse TrailLog map data", error);

      return;
    }

    if (!data.segments || data.segments.length === 0) {
      return;
    }

    const segments = data.segments
      .map((segment) => segment.points.map((point) => [point.lat, point.lon]))
      .filter((segment) => segment.length > 0);

    if (segments.length === 0) {
      return;
    }

    element.dataset.trackMapInitialized = "true";

    const map = L.map(element, {
      scrollWheelZoom: false,
    });

    const tileURL = element.dataset.trackTileUrl;

    const tileAttribution = element.dataset.trackTileAttribution;

    if (!tileURL) {
      console.error("TrailLog map tile URL is not configured");

      return;
    }

    L.tileLayer(tileURL, {
      maxZoom: 19,
      attribution: tileAttribution || "",
    }).addTo(map);

    const route = L.polyline(segments, {
      weight: 4,
      smoothFactor: 1,
    }).addTo(map);

    map.fitBounds(route.getBounds(), {
      padding: [20, 20],
      maxZoom: 16,
    });

    const firstSegment = segments[0];

    const lastSegment = segments[segments.length - 1];

    const start = firstSegment[0];

    const end = lastSegment[lastSegment.length - 1];

    L.circleMarker(start, {
      radius: 6,
    })
      .bindTooltip("Start")
      .addTo(map);

    L.circleMarker(end, {
      radius: 6,
    })
      .bindTooltip("Finish")
      .addTo(map);
  }

  document.addEventListener("DOMContentLoaded", () => {
    initializeMaps(document);
  });

  document.body.addEventListener("htmx:load", (event) => {
    initializeMaps(event.detail.elt);
  });
})();
