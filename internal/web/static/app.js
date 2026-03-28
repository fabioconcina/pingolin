(function () {
    "use strict";

    var SVG_NS = "http://www.w3.org/2000/svg";
    var eventSource = null;
    var gradientCounter = 0;

    function connect() {
        if (eventSource) {
            eventSource.close();
        }
        eventSource = new EventSource("/events");
        eventSource.onmessage = function (e) {
            try {
                updateDashboard(JSON.parse(e.data));
            } catch (err) {
                console.error("Failed to parse SSE data:", err);
            }
        };
        eventSource.onerror = function () {
            document.getElementById("status").textContent = "RECONNECTING...";
            document.getElementById("status").className = "status degraded";
        };
    }

    function updateDashboard(data) {
        // Status
        var statusEl = document.getElementById("status");
        statusEl.textContent = data.status.toUpperCase();
        statusEl.className = "status " + data.status;

        // Targets
        var container = document.getElementById("targets");
        container.innerHTML = "";
        if (data.targets) {
            data.targets.forEach(function (t) {
                container.appendChild(renderTarget(t));
            });
        }

        // DNS
        if (data.dns) {
            document.getElementById("dns-last").textContent =
                data.dns.last_ms != null ? data.dns.last_ms.toFixed(0) + "ms" : "--";
            document.getElementById("dns-avg").textContent =
                data.dns.avg_ms > 0 ? data.dns.avg_ms.toFixed(0) + "ms" : "--";
            var dnsInd = document.getElementById("dns-status");
            dnsInd.className = "indicator " + (data.dns.success ? "ok" : "fail");
            if (data.dns.resolver) {
                document.getElementById("dns-resolver").textContent = data.dns.resolver;
            }
        }

        // HTTP
        if (data.http) {
            var httpText = data.http.last_ms != null ? data.http.last_ms.toFixed(0) + "ms" : "--";
            if (data.http.last_ms != null && data.http.tls_ms) {
                httpText += " (tls " + data.http.tls_ms.toFixed(0) + "ms)";
            }
            document.getElementById("http-last").textContent = httpText;
            document.getElementById("http-status").textContent =
                data.http.status_code != null ? data.http.status_code : "--";
            var httpInd = document.getElementById("http-indicator");
            httpInd.className = "indicator " + (data.http.success ? "ok" : "fail");
            if (data.http.target) {
                document.getElementById("http-target").textContent = data.http.target;
            }
        }

        // Outages
        var list = document.getElementById("outages-list");
        if (!data.outages || data.outages.length === 0) {
            list.innerHTML = '<span class="empty">No outages recorded</span>';
        } else {
            list.innerHTML = "";
            data.outages.forEach(function (o) {
                var row = document.createElement("div");
                row.className = "outage-row";
                if (o.duration === "ongoing") {
                    row.className += " ongoing-row";
                }

                var timeSpan = document.createElement("span");
                timeSpan.className = "outage-time";
                timeSpan.textContent = o.started_at;

                var durSpan = document.createElement("span");
                durSpan.className = "outage-duration";
                durSpan.textContent = o.duration;

                var causeSpan = document.createElement("span");
                causeSpan.className = "outage-cause" + (o.duration === "ongoing" ? " ongoing" : "");
                causeSpan.textContent = o.cause;

                row.appendChild(timeSpan);
                row.appendChild(durSpan);
                row.appendChild(causeSpan);
                list.appendChild(row);
            });
        }

        // Updated at
        if (data.updated_at) {
            var d = new Date(data.updated_at);
            document.getElementById("updated-at").textContent = "updated " + d.toLocaleTimeString();
        }
    }

    function renderTarget(t) {
        var card = document.createElement("div");
        card.className = "card";

        var h3 = document.createElement("h3");
        h3.textContent = t.target;
        card.appendChild(h3);

        var stats = document.createElement("div");
        stats.className = "target-stats";

        stats.appendChild(makeStat("last", t.last_rtt != null ? t.last_rtt.toFixed(1) + "ms" : "--"));
        stats.appendChild(makeStat("avg", t.avg_rtt > 0 ? t.avg_rtt.toFixed(1) + "ms" : "--"));

        var lossStat = makeStat("loss", t.loss_pct.toFixed(1) + "%");
        var lossVal = lossStat.querySelector(".value");
        if (t.loss_pct > 5) {
            lossVal.className = "value loss-critical";
        } else if (t.loss_pct > 1) {
            lossVal.className = "value loss-warning";
        } else {
            lossVal.className = "value loss";
        }
        stats.appendChild(lossStat);

        if (t.jitter != null) {
            stats.appendChild(makeStat("jitter", t.jitter.toFixed(1) + "ms"));
        }

        card.appendChild(stats);

        // Sparkline
        if (t.sparkline && t.sparkline.length > 0) {
            var sparkDiv = document.createElement("div");
            sparkDiv.className = "sparkline-container";
            sparkDiv.appendChild(renderSparkline(t.sparkline, t.avg_rtt));
            attachTooltip(sparkDiv, t.sparkline);
            card.appendChild(sparkDiv);

            var axis = document.createElement("div");
            axis.className = "sparkline-axis";
            axis.innerHTML = '<span>-1h</span><span>now</span>';
            card.appendChild(axis);
        }

        return card;
    }

    function makeStat(label, value) {
        var stat = document.createElement("div");
        stat.className = "stat";

        var l = document.createElement("span");
        l.className = "label";
        l.textContent = label;

        var v = document.createElement("span");
        v.className = "value";
        v.textContent = value;

        stat.appendChild(l);
        stat.appendChild(v);
        return stat;
    }

    function renderSparkline(values, avg) {
        var w = 400;
        var h = 90;
        var svg = document.createElementNS(SVG_NS, "svg");
        svg.setAttribute("viewBox", "0 0 " + w + " " + h);
        svg.setAttribute("preserveAspectRatio", "none");

        if (values.length < 2) return svg;

        // Filter valid values for scale calculation
        var valid = values.filter(function (v) { return v > 0; });
        if (valid.length === 0) return svg;

        // Use 95th percentile as Y-axis cap
        var sorted = valid.slice().sort(function (a, b) { return a - b; });
        var p95 = sorted[Math.floor(sorted.length * 0.95)];
        var maxVal = Math.max(p95 * 1.5, avg * 3);
        if (maxVal === 0) maxVal = 1;

        var step = w / (values.length - 1);
        var padding = 4; // top/bottom padding in SVG units
        var plotH = h - padding * 2;

        function yPos(val) {
            return padding + plotH - (val / maxVal) * plotH;
        }

        // Build point arrays
        var points = [];      // {x, y, val} for valid points
        var lossIndices = [];  // indices where packet loss occurred

        for (var i = 0; i < values.length; i++) {
            var x = i * step;
            if (values[i] < 0) {
                lossIndices.push(i);
                points.push(null);
            } else {
                points.push({ x: x, y: yPos(values[i]), val: values[i] });
            }
        }

        // Gradient for area fill
        var gradId = "sparkGrad" + (++gradientCounter);
        var defs = document.createElementNS(SVG_NS, "defs");
        var grad = document.createElementNS(SVG_NS, "linearGradient");
        grad.setAttribute("id", gradId);
        grad.setAttribute("x1", "0"); grad.setAttribute("y1", "0");
        grad.setAttribute("x2", "0"); grad.setAttribute("y2", "1");
        var stop1 = document.createElementNS(SVG_NS, "stop");
        stop1.setAttribute("offset", "0%");
        stop1.setAttribute("stop-color", "rgba(63,185,80,0.18)");
        var stop2 = document.createElementNS(SVG_NS, "stop");
        stop2.setAttribute("offset", "100%");
        stop2.setAttribute("stop-color", "rgba(63,185,80,0)");
        grad.appendChild(stop1);
        grad.appendChild(stop2);
        defs.appendChild(grad);
        svg.appendChild(defs);

        // Build segments of consecutive valid points
        var segments = [];
        var current = [];
        for (var j = 0; j < points.length; j++) {
            if (points[j] !== null) {
                current.push(points[j]);
            } else {
                if (current.length > 0) {
                    segments.push(current);
                    current = [];
                }
            }
        }
        if (current.length > 0) segments.push(current);

        // Draw each segment
        segments.forEach(function (seg) {
            if (seg.length < 2) return;

            // Area fill path
            var areaD = "M" + seg[0].x + "," + h;
            for (var k = 0; k < seg.length; k++) {
                areaD += " L" + seg[k].x + "," + seg[k].y;
            }
            areaD += " L" + seg[seg.length - 1].x + "," + h + " Z";

            var areaPath = document.createElementNS(SVG_NS, "path");
            areaPath.setAttribute("d", areaD);
            areaPath.setAttribute("fill", "url(#" + gradId + ")");
            svg.appendChild(areaPath);

            // Line stroke - build colored sub-segments
            for (var k = 0; k < seg.length - 1; k++) {
                var line = document.createElementNS(SVG_NS, "line");
                line.setAttribute("x1", seg[k].x);
                line.setAttribute("y1", seg[k].y);
                line.setAttribute("x2", seg[k + 1].x);
                line.setAttribute("y2", seg[k + 1].y);
                line.setAttribute("stroke-width", "1.8");
                line.setAttribute("stroke-linecap", "round");

                var segMax = Math.max(seg[k].val, seg[k + 1].val);
                if (avg > 0 && segMax > 5 * avg) {
                    line.setAttribute("stroke", "#f85149");
                } else if (avg > 0 && segMax > 2 * avg) {
                    line.setAttribute("stroke", "#d29922");
                } else {
                    line.setAttribute("stroke", "#3fb950");
                }
                svg.appendChild(line);
            }
        });

        // Red dots for packet loss
        lossIndices.forEach(function (idx) {
            var dot = document.createElementNS(SVG_NS, "circle");
            dot.setAttribute("cx", idx * step);
            dot.setAttribute("cy", h - padding);
            dot.setAttribute("r", 2.5);
            dot.setAttribute("fill", "#f85149");
            svg.appendChild(dot);
        });

        // Average baseline
        if (avg > 0) {
            var avgY = yPos(avg);
            var baseline = document.createElementNS(SVG_NS, "line");
            baseline.setAttribute("x1", 0);
            baseline.setAttribute("y1", avgY);
            baseline.setAttribute("x2", w);
            baseline.setAttribute("y2", avgY);
            baseline.setAttribute("stroke", "#484f58");
            baseline.setAttribute("stroke-width", "0.5");
            baseline.setAttribute("stroke-dasharray", "4,4");
            svg.appendChild(baseline);
        }

        return svg;
    }

    function attachTooltip(container, values) {
        var tooltip = document.createElement("div");
        tooltip.className = "sparkline-tooltip";
        container.appendChild(tooltip);

        container.addEventListener("mousemove", function (e) {
            var rect = container.getBoundingClientRect();
            var x = e.clientX - rect.left;
            var pct = x / rect.width;
            var idx = Math.round(pct * (values.length - 1));
            idx = Math.max(0, Math.min(idx, values.length - 1));

            var val = values[idx];
            if (val < 0) {
                tooltip.textContent = "loss";
            } else {
                tooltip.textContent = val.toFixed(1) + "ms";
            }
            tooltip.style.left = x + "px";
            tooltip.style.opacity = "1";
        });

        container.addEventListener("mouseleave", function () {
            tooltip.style.opacity = "0";
        });
    }

    // Start
    connect();
})();
