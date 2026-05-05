#!/usr/bin/env python3
"""
HOWL README Diagrams — OpsDB README
8 figures covering the README's operational model, runner model,
governance path, and audit-native architecture.
Output: PNG files to ../figures/
"""

import matplotlib

matplotlib.use("Agg")
import matplotlib.pyplot as plt
import matplotlib.patches as mpatches
import numpy as np
import os

outdir = os.path.join(
    os.path.dirname(os.path.abspath(__file__)), "..", "figures"
)
os.makedirs(outdir, exist_ok=True)

# Light mode
if True:
    # ── Global palette (Kindle / light mode) ──
    BG      = '#ffffff'
    PAN     = '#f0ede8'
    GOLD    = '#a07820'
    SILVER  = '#505860'
    CYAN    = '#1a8a80'
    MAG     = '#a03058'
    BLUE    = '#2855a0'
    GREEN   = '#2a7a3a'
    RED     = '#b82020'
    ORANGE  = '#c06a18'
    WHITE   = '#1a1a22'
    DIM     = '#908e88'
    PURPLE  = '#6040a0'
else:
    # ── Global palette (D7.2) ──
    BG      = '#0a0a12'
    PAN     = '#12121f'
    GOLD    = '#d4a843'
    SILVER  = '#a0a8b8'
    CYAN    = '#4ecdc4'
    MAG     = '#c74b7a'
    BLUE    = '#5b8def'
    GREEN   = '#6bcf7f'
    RED     = '#e05555'
    ORANGE  = '#e8944a'
    WHITE   = '#e8e8f0'
    DIM     = '#555570'
    PURPLE  = '#9b7bd4'

def style_ax(ax):
    ax.set_facecolor(PAN)
    for spine in ax.spines.values():
        spine.set_color(DIM)
        spine.set_linewidth(0.5)
    ax.tick_params(colors=DIM, labelsize=9)
    ax.title.set_color(WHITE)
    ax.xaxis.label.set_color(SILVER)
    ax.yaxis.label.set_color(SILVER)
    ax.grid(color=DIM, alpha=0.15, linewidth=0.5)


def save(fig, filename):
    path = os.path.join(outdir, filename)
    fig.savefig(
        path, dpi=180, facecolor=BG, bbox_inches="tight", pad_inches=0.3
    )


def add_box(
    ax,
    x,
    y,
    w,
    h,
    text,
    edge,
    face=PAN,
    txt=WHITE,
    lw=1.8,
    rounding=0.04,
    fs=10,
):
    patch = mpatches.FancyBboxPatch(
        (x, y),
        w,
        h,
        boxstyle="round,pad=0.02,rounding_size=%0.3f" % rounding,
        linewidth=lw,
        edgecolor=edge,
        facecolor=face,
    )
    ax.add_patch(patch)
    ax.text(
        x + w / 2.0,
        y + h / 2.0,
        text,
        color=txt,
        fontsize=fs,
        ha="center",
        va="center",
        weight="bold",
    )


def add_arrow(ax, x1, y1, x2, y2, color, lw=2.0, style="-|>"):
    ax.annotate(
        "",
        xy=(x2, y2),
        xytext=(x1, y1),
        arrowprops=dict(arrowstyle=style, color=color, lw=lw),
    )


saved = []

# ================================================================
# FIG 1: CLOSED OPERATIONAL LOOP
# Type: 7 Progression/Sequence Diagram
# Shows: the system as a repeating operational loop that unifies
# humans, automation, and auditors through the same cycle.
# ================================================================
fig, ax = plt.subplots(figsize=(16, 10))
fig.patch.set_facecolor(BG)
ax.set_facecolor(PAN)
ax.set_xlim(0, 10)
ax.set_ylim(0, 10)
ax.axis("off")

center = (5.0, 5.2)
r = 3.0
angles = np.linspace(0.0, 2.0 * np.pi, 7)[:-1] + np.pi / 6.0
labels = [
    ("Observe", CYAN),
    ("Validate", BLUE),
    ("Approve", ORANGE),
    ("Execute", GREEN),
    ("Record", MAG),
    ("Query", PURPLE),
]

for i, item in enumerate(labels):
    label, color = item
    x = center[0] + r * np.cos(angles[i])
    y = center[1] + r * np.sin(angles[i])
    circ = mpatches.Circle(
        (x, y), 0.85, facecolor=PAN, edgecolor=color, linewidth=2.2
    )
    ax.add_patch(circ)
    ax.text(
        x,
        y,
        label,
        color=WHITE,
        fontsize=11,
        ha="center",
        va="center",
        weight="bold",
    )

for i in range(len(labels)):
    a1 = angles[i]
    a2 = angles[(i + 1) % len(labels)]
    x1 = center[0] + 2.15 * np.cos(a1)
    y1 = center[1] + 2.15 * np.sin(a1)
    x2 = center[0] + 2.15 * np.cos(a2)
    y2 = center[1] + 2.15 * np.sin(a2)
    add_arrow(ax, x1, y1, x2, y2, GOLD, lw=2.2)

core = mpatches.Circle(center, 1.15, facecolor=BG, edgecolor=GOLD, linewidth=2.5)
ax.add_patch(core)
ax.text(
    center[0],
    center[1],
    "OpsDB",
    color=GOLD,
    fontsize=15,
    ha="center",
    va="center",
    weight="bold",
)

user_pos = [(1.5, 8.2), (8.5, 8.2), (5.0, 1.3)]
user_labels = [
    ("Humans", WHITE),
    ("Automation", CYAN),
    ("Auditors", MAG),
]
for i, item in enumerate(user_labels):
    label, color = item
    x, y = user_pos[i]
    e = mpatches.Ellipse(
        (x, y), 2.0, 0.9, facecolor=PAN, edgecolor=color, linewidth=1.8
    )
    ax.add_patch(e)
    ax.text(
        x, y, label, color=color, fontsize=11, ha="center", va="center"
    )
    add_arrow(ax, x, y - 0.6 if y > 5 else y + 0.6, center[0], center[1], DIM, lw=1.2)

ax.text(
    5.0,
    9.45 + 0.5,
    "OpsDB as a Closed Operational Loop",
    color=GOLD,
    fontsize=16,
    ha="center",
    va="center",
    weight="bold",
)
ax.text(
    5.0,
    8.95 + 0.5,
    "Operate, govern, and evidence through the same motion.",
    color=SILVER,
    fontsize=10,
    ha="center",
    va="center",
)

filename = "opsdb_01_operational_loop.png"
save(fig, filename)
saved.append(filename)
print("  Saved: %s" % filename)
plt.close(fig)

# ================================================================
# FIG 2: DEFINITIONS BECOME RUNTIME CONTROL
# Type: 7 Progression/Sequence Diagram
# Shows: how static definitions are transformed into live runtime
# structure and gate behavior.
# ================================================================
fig, ax = plt.subplots(figsize=(18, 8))
fig.patch.set_facecolor(BG)
ax.set_facecolor(PAN)
ax.set_xlim(0, 18)
ax.set_ylim(0, 8)
ax.axis("off")

steps = [
    (0.8, 2.5, 2.4, 3.0, "Schema\nDefinitions", BLUE),
    (4.0, 2.5, 2.4, 3.0, "Loader\nValidate\nResolve", CYAN),
    (7.2, 2.5, 2.4, 3.0, "Postgres\nTables", GREEN),
    (10.4, 2.5, 2.8, 3.0, "_schema_*\nMetadata", PURPLE),
    (14.0, 2.5, 2.8, 3.0, "API Gate\nRuntime Rules", GOLD),
]

for item in steps:
    x, y, w, h, label, color = item
    add_box(ax, x, y, w, h, label, color, fs=11)

for i in range(len(steps) - 1):
    x1 = steps[i][0] + steps[i][2]
    y1 = steps[i][1] + steps[i][3] / 2.0
    x2 = steps[i + 1][0]
    y2 = steps[i + 1][1] + steps[i + 1][3] / 2.0
    add_arrow(ax, x1 + 0.15, y1, x2 - 0.15, y2, GOLD, lw=2.2)

ax.text(
    9.0,
    6.9,
    "Definitions Become Runtime Control",
    color=GOLD,
    fontsize=16,
    ha="center",
    weight="bold",
)
ax.text(
    9.0,
    6.35,
    "The schema is not documentation beside the system; it becomes the system.",
    color=SILVER,
    fontsize=10,
    ha="center",
)

filename = "opsdb_02_schema_to_runtime.png"
save(fig, filename)
saved.append(filename)
print("  Saved: %s" % filename)
plt.close(fig)

# ================================================================
# FIG 3: GOVERNED CHANGE AS STAGED MOTION
# Type: 7 Progression/Sequence Diagram
# Shows: the separation of intent, approval, execution, and durable
# version history.
# ================================================================
fig, ax = plt.subplots(figsize=(18, 8))
fig.patch.set_facecolor(BG)
ax.set_facecolor(PAN)
ax.set_xlim(0, 18)
ax.set_ylim(0, 8)
ax.axis("off")

stage_x = [1.0, 4.5, 8.0, 11.5, 15.0]
stage_data = [
    ("Intent", BLUE, 1.3),
    ("Approvals", ORANGE, 2.3),
    ("Executor", CYAN, 3.2),
    ("Apply", GREEN, 4.2),
    ("Version\nHistory", MAG, 5.2),
]

for i, item in enumerate(stage_data):
    label, color, y = item
    add_box(ax, stage_x[i], y, 2.0, 1.2, label, color, fs=11)
    if i < len(stage_data) - 1:
        add_arrow(
            ax,
            stage_x[i] + 2.1,
            y + 0.6,
            stage_x[i + 1] - 0.1,
            stage_data[i + 1][2] + 0.6,
            GOLD,
            lw=2.2,
        )

timeline_y = 1.2
ax.plot([1.0, 17.0], [timeline_y, timeline_y], color=DIM, lw=1.4)
for x in stage_x:
    ax.plot([x + 1.0, x + 1.0], [timeline_y - 0.15, timeline_y + 0.15], color=DIM, lw=1.2)

ax.text(
    9.0,
    6.9,
    "Governed Change as Staged Motion",
    color=GOLD,
    fontsize=16,
    ha="center",
    weight="bold",
)
ax.text(
    9.0,
    6.35,
    "A change is proposed, routed, executed, and preserved — never collapsed into one opaque edit.",
    color=SILVER,
    fontsize=10,
    ha="center",
)

filename = "opsdb_03_governed_change_flow.png"
save(fig, filename)
saved.append(filename)
print("  Saved: %s" % filename)
plt.close(fig)

# ================================================================
# FIG 4: PROVENANCE THROUGH TIME
# Type: 1 Running/Convergence Chart
# Shows: how one operational event leaves aligned traces across
# governance, execution, audit, and history over time.
# ================================================================
fig, ax = plt.subplots(figsize=(16, 10))
fig.patch.set_facecolor(BG)
style_ax(ax)
ax.set_xlim(0, 10)
ax.set_ylim(0, 5)

tracks = [
    ("Intent", 4.2, BLUE),
    ("Approval", 3.2, ORANGE),
    ("Execution", 2.2, GREEN),
    ("Audit", 1.2, MAG),
]
for label, y, color in tracks:
    ax.hlines(y, 0.5, 9.5, colors=DIM, linewidth=0.8)
    ax.text(0.2, y, label, color=color, fontsize=10, ha="left", va="center")

events = [
    (1.0, 4.2, "submit", BLUE),
    (3.0, 3.2, "approve", ORANGE),
    (5.3, 2.2, "apply", GREEN),
    (7.4, 1.2, "query", PURPLE),
]
for x, y, label, color in events:
    ax.scatter(
        [x],
        [y],
        s=180,
        color=color,
        edgecolors=WHITE,
        linewidth=1.6,
        zorder=3,
    )
    ax.text(x, y + 0.28, label, color=color, fontsize=9, ha="center")

add_arrow(ax, 1.0, 4.0, 3.0, 3.4, GOLD, lw=1.8)
add_arrow(ax, 3.0, 3.0, 5.3, 2.4, GOLD, lw=1.8)
add_arrow(ax, 5.3, 2.0, 7.4, 1.4, GOLD, lw=1.8)

ax.axvspan(5.0, 5.6, color=GREEN, alpha=0.08)
ax.axvspan(2.7, 3.3, color=ORANGE, alpha=0.08)
ax.axvspan(0.8, 1.2, color=BLUE, alpha=0.08)

ax.set_xticks([1.0, 3.0, 5.3, 7.4, 9.0])
ax.set_xticklabels(
    ["T0", "T1", "T2", "T3", "Later"], color=DIM, fontsize=9
)
ax.set_yticks([])
ax.set_title(
    "Provenance Through Time",
    color=GOLD,
    fontsize=16,
    weight="bold",
    pad=16,
)
ax.set_xlabel("Operational time", fontsize=11, color=SILVER)
ax.text(
    5.0,
    4.72,
    "One action becomes a queryable timeline rather than a reconstructed story.",
    color=SILVER,
    fontsize=10,
    ha="center",
)

filename = "opsdb_04_provenance_timeline.png"
save(fig, filename)
saved.append(filename)
print("  Saved: %s" % filename)
plt.close(fig)

# ================================================================
# FIG 5: AUTOMATION TOPOLOGY SHIFT
# Type: 4 Geometric Cross-Section
# Shows: mesh-coupled automation on the left and hub-coordinated
# automation on the right.
# ================================================================
fig, (ax1, ax2) = plt.subplots(
    1, 2, figsize=(18, 9), gridspec_kw={"wspace": 0.30}
)
fig.patch.set_facecolor(BG)

for ax in [ax1, ax2]:
    ax.set_facecolor(PAN)
    ax.set_xlim(0, 10)
    ax.set_ylim(0, 10)
    ax.axis("off")

pts = np.array(
    [
        [2.0, 7.5],
        [5.0, 8.4],
        [8.0, 7.0],
        [7.0, 3.5],
        [3.2, 2.5],
        [1.5, 4.8],
    ]
)

# Left: mesh
for i in range(len(pts)):
    for j in range(i + 1, len(pts)):
        if (i + j) % 2 == 0:
            ax1.plot(
                [pts[i, 0], pts[j, 0]],
                [pts[i, 1], pts[j, 1]],
                color=DIM,
                lw=1.0,
                alpha=0.7,
            )
for i in range(len(pts)):
    c = mpatches.Circle(
        (pts[i, 0], pts[i, 1]), 0.42, facecolor=PAN, edgecolor=RED, linewidth=1.8
    )
    ax1.add_patch(c)
    ax1.text(
        pts[i, 0],
        pts[i, 1],
        "R%d" % (i + 1),
        color=WHITE,
        fontsize=9,
        ha="center",
        va="center",
    )
ax1.text(
    5.0,
    9.3,
    "Directly Coupled Automation",
    color=RED,
    fontsize=14,
    ha="center",
    weight="bold",
)

# Right: hub
hub = (5.0, 5.3)
hub_patch = mpatches.Circle(
    hub, 1.0, facecolor=BG, edgecolor=GOLD, linewidth=2.4
)
ax2.add_patch(hub_patch)
ax2.text(
    hub[0], hub[1], "OpsDB", color=GOLD, fontsize=13, ha="center", va="center", weight="bold"
)
for i in range(len(pts)):
    c = mpatches.Circle(
        (pts[i, 0], pts[i, 1]), 0.42, facecolor=PAN, edgecolor=CYAN, linewidth=1.8
    )
    ax2.add_patch(c)
    ax2.text(
        pts[i, 0],
        pts[i, 1],
        "R%d" % (i + 1),
        color=WHITE,
        fontsize=9,
        ha="center",
        va="center",
    )
    add_arrow(ax2, pts[i, 0], pts[i, 1], hub[0], hub[1], CYAN, lw=1.4)
ax2.text(
    5.0,
    9.3,
    "Material-State Coordination",
    color=CYAN,
    fontsize=14,
    ha="center",
    weight="bold",
)

fig.text(
    0.5,
    0.95,
    "Automation Topology Shift",
    color=GOLD,
    fontsize=16,
    ha="center",
    weight="bold",
)
fig.text(
    0.5,
    0.915,
    "Runners stop calling each other and coordinate through shared fresh state.",
    color=SILVER,
    fontsize=10,
    ha="center",
)

filename = "opsdb_05_automation_topology.png"
save(fig, filename)
saved.append(filename)
print("  Saved: %s" % filename)
plt.close(fig)

# ================================================================
# FIG 6: ONE OPERATIONAL GRAMMAR ACROSS DOMAINS
# Type: 2 Scale/Landscape Diagram
# Shows: diverse operational domains placed on one shared grammar.
# ================================================================
fig, ax = plt.subplots(figsize=(18, 8))
fig.patch.set_facecolor(BG)
style_ax(ax)
ax.set_xlim(0, 100)
ax.set_ylim(0, 10)
ax.set_yticks([])
ax.set_xticks([])
for spine in ax.spines.values():
    spine.set_visible(False)

ax.hlines(5.0, 5, 95, colors=DIM, linewidth=1.4)

domains = [
    (14, "Cloud", BLUE, 7.2),
    (34, "Kubernetes", CYAN, 7.8),
    (56, "Internal\nServices", GREEN, 7.0),
    (76, "Manual\nProcedures", ORANGE, 7.5),
    (90, "Audit", MAG, 6.8),
]
for x, label, color, y in domains:
    ax.vlines(x, 4.2, 5.8, colors=color, linewidth=2.2)
    ax.scatter(
        [x],
        [5.0],
        s=180,
        color=color,
        edgecolors=WHITE,
        linewidth=1.6,
        zorder=3,
    )
    ax.text(x, y, label, color=color, fontsize=11, ha="center", va="center")

bands = [
    (8, 22, BLUE, "Observe"),
    (24, 42, CYAN, "Validate"),
    (44, 62, ORANGE, "Approve"),
    (64, 82, GREEN, "Execute"),
    (84, 94, MAG, "Record / Query"),
]
for x1, x2, color, label in bands:
    rect = mpatches.FancyBboxPatch(
        (x1, 2.0),
        x2 - x1,
        1.1,
        boxstyle="round,pad=0.02,rounding_size=0.15",
        linewidth=1.2,
        edgecolor=color,
        facecolor=color,
        alpha=0.10,
    )
    ax.add_patch(rect)
    ax.text(
        (x1 + x2) / 2.0,
        2.55,
        label,
        color=color,
        fontsize=10,
        ha="center",
        va="center",
        weight="bold",
    )

ax.set_title(
    "One Operational Grammar Across Domains",
    color=GOLD,
    fontsize=16,
    weight="bold",
    pad=16,
)
ax.text(
    50,
    9.1,
    "Different substrates; one model of observe, govern, act, and preserve.",
    color=SILVER,
    fontsize=10,
    ha="center",
)

filename = "opsdb_06_operational_landscape.png"
save(fig, filename)
saved.append(filename)
print("  Saved: %s" % filename)
plt.close(fig)

# ================================================================
# FIG 7: THE API GATE AS CONTROL MEMBRANE
# Type: 4 Geometric Cross-Section
# Shows: external requests crossing one membrane where auth,
# validation, policy, and audit are applied in order.
# ================================================================
fig, ax = plt.subplots(figsize=(18, 10))
fig.patch.set_facecolor(BG)
ax.set_facecolor(PAN)
ax.set_xlim(0, 18)
ax.set_ylim(0, 10)
ax.axis("off")

left_sources = [
    (1.8, 7.8, "Human", WHITE),
    (1.8, 5.2, "Runner", CYAN),
    (1.8, 2.6, "Importer", BLUE),
]
for x, y, label, color in left_sources:
    e = mpatches.Ellipse(
        (x, y), 2.1, 0.95, facecolor=PAN, edgecolor=color, linewidth=1.8
    )
    ax.add_patch(e)
    ax.text(x, y, label, color=color, fontsize=11, ha="center", va="center")
    add_arrow(ax, x + 1.15, y, 5.0, y, color, lw=1.6)

membrane_x = 6.0
membrane = mpatches.FancyBboxPatch(
    (membrane_x, 1.0),
    5.5,
    8.0,
    boxstyle="round,pad=0.02,rounding_size=0.08",
    linewidth=2.0,
    edgecolor=GOLD,
    facecolor=BG,
)
ax.add_patch(membrane)

layers = [
    ("Authenticate", 8.1, BLUE),
    ("Authorize", 6.8, CYAN),
    ("Validate", 5.5, ORANGE),
    ("Policy", 4.2, PURPLE),
    ("Audit", 2.9, MAG),
]
for label, y, color in layers:
    ax.hlines(y, 6.5, 11.0, colors=DIM, linewidth=0.8)
    ax.text(8.75, y + 0.28, label, color=color, fontsize=10, ha="center")

target = mpatches.FancyBboxPatch(
    (13.1, 3.3),
    3.2,
    3.2,
    boxstyle="round,pad=0.02,rounding_size=0.08",
    linewidth=2.0,
    edgecolor=GREEN,
    facecolor=PAN,
)
ax.add_patch(target)
ax.text(
    14.7,
    4.9,
    "Durable\nState",
    color=GREEN,
    fontsize=13,
    ha="center",
    va="center",
    weight="bold",
)
add_arrow(ax, 11.7, 5.0, 13.0, 5.0, GREEN, lw=2.2)

ax.text(
    9.0,
    9.45,
    "The API Gate as a Control Membrane",
    color=GOLD,
    fontsize=16,
    ha="center",
    weight="bold",
)
ax.text(
    9.0,
    8.95,
    "Many callers, one ordered boundary before state can change.",
    color=SILVER,
    fontsize=10,
    ha="center",
)

filename = "opsdb_07_api_membrane.png"
save(fig, filename)
saved.append(filename)
print("  Saved: %s" % filename)
plt.close(fig)

# ================================================================
# FIG 8: OPERATIONS AND AUDIT AS ONE SYSTEM
# Type: 5 Connection/Integer Map
# Shows: operational events connected directly to their evidence
# artifacts instead of requiring later reconstruction.
# ================================================================
fig, ax = plt.subplots(figsize=(18, 10))
fig.patch.set_facecolor(BG)
ax.set_facecolor(PAN)
ax.set_xlim(0, 18)
ax.set_ylim(0, 10)
ax.axis("off")

ops_nodes = [
    (3.0, 7.7, "Change\nIntent", BLUE),
    (3.0, 5.4, "Approval", ORANGE),
    (3.0, 3.1, "Execution", GREEN),
]
audit_nodes = [
    (15.0, 7.7, "Audit\nEntry", MAG),
    (15.0, 5.4, "Version\nHistory", PURPLE),
    (15.0, 3.1, "Queryable\nState", GOLD),
]

for x, y, label, color in ops_nodes:
    add_box(ax, x - 1.2, y - 0.55, 2.4, 1.1, label, color, fs=10)
for x, y, label, color in audit_nodes:
    add_box(ax, x - 1.3, y - 0.55, 2.6, 1.1, label, color, fs=10)

center = (9.0, 5.4)
ring = mpatches.Circle(
    center, 1.35, facecolor=BG, edgecolor=GOLD, linewidth=2.4
)
ax.add_patch(ring)
ax.text(
    center[0],
    center[1],
    "Same\nSystem",
    color=GOLD,
    fontsize=13,
    ha="center",
    va="center",
    weight="bold",
)

for x, y, label, color in ops_nodes:
    add_arrow(ax, x + 1.25, y, center[0] - 1.45, center[1], color, lw=1.8)
for x, y, label, color in audit_nodes:
    add_arrow(ax, center[0] + 1.45, center[1], x - 1.35, y, color, lw=1.8)

ax.text(
    9.0,
    9.35,
    "Operations and Audit as One System",
    color=GOLD,
    fontsize=16,
    ha="center",
    weight="bold",
)
ax.text(
    9.0,
    8.85,
    "Operational events are evidence-bearing by construction, not by later collection.",
    color=SILVER,
    fontsize=10,
    ha="center",
)

filename = "opsdb_08_operations_audit_unified.png"
save(fig, filename)
saved.append(filename)
print("  Saved: %s" % filename)
plt.close(fig)

print("")
print("Generated files:")
for name in saved:
    print(" - %s" % name)
