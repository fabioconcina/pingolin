(function () {
    "use strict";

    let currentRange = "1h";
    let eventSource = null;

    function connect() {
        if (eventSource) {
            eventSource.close();
        }
        eventSource = new EventSource("/events?range=" + currentRange);
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
                container.appendChild(renderTarget(t, data.time_range));
            });
        }

        // DNS
        if (data.dns) {
            document.getElementById("dns-last").textContent = data.dns.last_ms != null ? data.dns.last_ms.toFixed(0) + "ms" : "--";
            document.getElementById("dns-avg").textContent = data.dns.avg_ms > 0 ? data.dns.avg_ms.toFixed(0) + "ms" : "--";
            var dnsInd = document.getElementById("dns-status");
            dnsInd.className = "indicator " + (data.dns.success ? "ok" : "fail");
        }

        // HTTP
        if (data.http) {
            document.getElementById("http-last").textContent = data.http.last_ms != null ? data.http.last_ms.toFixed(0) + "ms" : "--";
            document.getElementById("http-status").textContent = data.http.status_code != null ? data.http.status_code : "--";
            var httpInd = document.getElementById("http-indicator");
            httpInd.className = "indicator " + (data.http.success ? "ok" : "fail");
        }

        // Outages
        var list = document.getElementById("outages-list");
        if (!data.outages || data.outages.length === 0) {
            list.innerHTML = '<span class="dim">No outages recorded</span>';
        } else {
            list.innerHTML = "";
            data.outages.forEach(function (o) {
                var row = document.createElement("div");
                row.className = "outage-row";

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
            card.appendChild(sparkDiv);
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
        var h = 50;
        var svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
        svg.setAttribute("viewBox", "0 0 " + w + " " + h);
        svg.setAttribute("preserveAspectRatio", "none");

        if (values.length < 2) return svg;

        // Filter valid values for scale calculation
        var valid = values.filter(function (v) { return v > 0; });
        if (valid.length === 0) return svg;

        // Use 95th percentile as Y-axis cap so spikes don't crush normal data
        var sorted = valid.slice().sort(function (a, b) { return a - b; });
        var p95 = sorted[Math.floor(sorted.length * 0.95)];
        var maxVal = Math.max(p95 * 1.5, avg * 3);
        if (maxVal === 0) maxVal = 1;

        var step = w / (values.length - 1);

        // Build path segments colored by threshold
        for (var i = 0; i < values.length - 1; i++) {
            var v1 = values[i];
            var v2 = values[i + 1];

            // Skip loss segments
            if (v1 < 0 || v2 < 0) {
                if (v1 < 0) {
                    // Draw a red dot for packet loss
                    var dot = document.createElementNS("http://www.w3.org/2000/svg", "circle");
                    dot.setAttribute("cx", i * step);
                    dot.setAttribute("cy", h - 4);
                    dot.setAttribute("r", 2);
                    dot.setAttribute("fill", "#e74c3c");
                    svg.appendChild(dot);
                }
                continue;
            }

            var x1 = i * step;
            var y1 = h - (v1 / maxVal) * h;
            var x2 = (i + 1) * step;
            var y2 = h - (v2 / maxVal) * h;

            var line = document.createElementNS("http://www.w3.org/2000/svg", "line");
            line.setAttribute("x1", x1);
            line.setAttribute("y1", y1);
            line.setAttribute("x2", x2);
            line.setAttribute("y2", y2);
            line.setAttribute("stroke-width", "1.5");

            // Color based on thresholds (same as TUI: >5x avg = red, >2x avg = yellow)
            var segMax = Math.max(v1, v2);
            if (avg > 0 && segMax > 5 * avg) {
                line.setAttribute("stroke", "#e74c3c");
            } else if (avg > 0 && segMax > 2 * avg) {
                line.setAttribute("stroke", "#f1c40f");
            } else {
                line.setAttribute("stroke", "#2ecc71");
            }

            svg.appendChild(line);
        }

        // Average baseline
        if (avg > 0) {
            var avgY = h - (avg / maxVal) * h;
            var baseline = document.createElementNS("http://www.w3.org/2000/svg", "line");
            baseline.setAttribute("x1", 0);
            baseline.setAttribute("y1", avgY);
            baseline.setAttribute("x2", w);
            baseline.setAttribute("y2", avgY);
            baseline.setAttribute("stroke", "#4a4a6a");
            baseline.setAttribute("stroke-width", "0.5");
            baseline.setAttribute("stroke-dasharray", "4,4");
            svg.appendChild(baseline);
        }

        return svg;
    }

    // Time range buttons
    document.getElementById("time-ranges").addEventListener("click", function (e) {
        if (e.target.tagName !== "BUTTON") return;

        var buttons = document.querySelectorAll(".time-ranges button");
        buttons.forEach(function (b) { b.classList.remove("active"); });
        e.target.classList.add("active");

        currentRange = e.target.getAttribute("data-range");
        connect();
    });

    // Start
    connect();
})();
