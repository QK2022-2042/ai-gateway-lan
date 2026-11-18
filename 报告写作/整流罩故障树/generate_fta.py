# -*- coding: utf-8 -*-
"""生成国军标（GJB）格式的整流罩质量问题故障树 SVG。

符号约定：
  矩形 = 事件（顶事件/中间事件）
  圆形 = 底事件
  或门 = 弧形底 + 尖顶的逻辑门符号
  连线 = 因果连接，无箭头
"""

W, H = 1900, 1060
FONT = "STHeiti, 'Heiti SC', 'PingFang SC', 'Microsoft YaHei', sans-serif"

TOP = ("T", "整流罩质量问题")
BRANCHES = [
    {
        "id": "M1", "name": "设计问题", "x": 340,
        "basic": ("X1", "设计问题"),
        "events": [
            ("M1-1", "结构强度不足"),
            ("M1-2", "刚度设计不足"),
            ("M1-3", "连接区设计不合理"),
            ("M1-4", "尺寸链设计超差"),
            ("M1-5", "材料选型不当"),
            ("M1-6", "分离机构设计缺陷"),
        ],
    },
    {
        "id": "M2", "name": "装配工艺问题", "x": 950,
        "basic": ("X2", "工艺问题"),
        "events": [
            ("M2-1", "装配间隙超差"),
            ("M2-2", "紧固件预紧力不足"),
            ("M2-3", "胶接界面脱粘"),
            ("M2-4", "密封性能不达标"),
            ("M2-5", "形位公差超差"),
            ("M2-6", "复材分层孔隙超标"),
        ],
    },
    {
        "id": "M3", "name": "表观问题", "x": 1560,
        "basic": ("X3", "现场操作问题"),
        "events": [
            ("M3-1", "表面划伤"),
            ("M3-2", "凹坑压痕"),
            ("M3-3", "涂层脱落起泡"),
            ("M3-4", "表面色差"),
            ("M3-5", "纤维外露"),
            ("M3-6", "表面污染"),
        ],
    },
]

# 纵向坐标
TOP_CY, TOP_W, TOP_H = 92, 320, 64
G0_CY, GATE_W, GATE_H = 186, 76, 62
BUS_Y = 250
M_CY, M_W, M_H = 306, 260, 60
G_CY = 398
ROW_CYS = [570, 670, 770]
NODE_W, NODE_H = 215, 56
COL_OFF = 150
X_CY, X_R = 890, 46

out = []
add = out.append


def line(x1, y1, x2, y2, cls="conn"):
    add(f'<line class="{cls}" x1="{x1}" y1="{y1}" x2="{x2}" y2="{y2}"/>')


def rect(cx, cy, w, h, cls):
    add(f'<rect class="{cls}" x="{cx - w / 2}" y="{cy - h / 2}" width="{w}" height="{h}" rx="2"/>')


def text(x, y, s, size, cls="label", anchor="middle", weight=None):
    w = f' font-weight="{weight}"' if weight else ""
    add(f'<text class="{cls}" x="{x}" y="{y}" font-size="{size}" text-anchor="{anchor}" '
        f'dominant-baseline="central"{w}>{s}</text>')


def or_gate(cx, cy, w=GATE_W, h=GATE_H):
    """IEC/GJB 或门：底部内凹，两侧外凸并在顶部收成尖角。"""
    hw, hh = w / 2, h / 2
    top, bot = cy - hh, cy + hh
    d = (f"M {cx - hw:.1f},{bot:.1f} "
         f"Q {cx:.1f},{bot - hh * 0.95:.1f} {cx + hw:.1f},{bot:.1f} "
         f"Q {cx + hw * 0.70:.1f},{cy + hh * 0.05:.1f} {cx:.1f},{top:.1f} "
         f"Q {cx - hw * 0.70:.1f},{cy + hh * 0.05:.1f} {cx - hw:.1f},{bot:.1f} Z")
    add(f'<path class="gate" d="{d}"/>')


add(f'<svg xmlns="http://www.w3.org/2000/svg" width="{W}" height="{H}" viewBox="0 0 {W} {H}">')
add("""<style>
  .conn { stroke:#222; stroke-width:1.6; fill:none; stroke-linecap:square; }
  .gate { stroke:#111; stroke-width:1.8; fill:#fff; }
  .ev-top { stroke:#111; stroke-width:2.4; fill:#fff; }
  .ev-mid { stroke:#111; stroke-width:1.7; fill:#f4f5f7; }
  .ev-ph { stroke:#333; stroke-width:1.4; fill:#fff; }
  .ev-basic { stroke:#111; stroke-width:1.9; fill:#fff; }
  text { font-family:%s; fill:#111; }
  .label { font-weight:400; }
  .strong { font-weight:700; }
  .caption { fill:#333; }
  .legend { fill:#333; font-size:14px; }
</style>""" % FONT)

# 标题
text(950, 32, "整流罩质量问题故障树", 26, weight="700")

# ---- 顶事件到 G0 ----
tx, ty = 950, TOP_CY
line(tx, ty + TOP_H / 2, tx, G0_CY - GATE_H / 2)
rect(tx, ty, TOP_W, TOP_H, "ev-top")
text(tx, ty, f"{TOP[0]}  {TOP[1]}", 21, weight="700")
or_gate(tx, G0_CY)
text(tx + 58, G0_CY + 4, "G0", 14, anchor="start")

# ---- G0 到三个中间事件 ----
line(tx, G0_CY + GATE_H / 2, tx, BUS_Y)
line(BRANCHES[0]["x"], BUS_Y, BRANCHES[-1]["x"], BUS_Y)
for b in BRANCHES:
    line(b["x"], BUS_Y, b["x"], M_CY - M_H / 2)
    rect(b["x"], M_CY, M_W, M_H, "ev-mid")
    text(b["x"], M_CY, f'{b["id"]}  {b["name"]}', 19)
    line(b["x"], M_CY + M_H / 2, b["x"], G_CY - GATE_H / 2)
    or_gate(b["x"], G_CY)
    text(b["x"] + 58, G_CY + 4, b["id"].replace("M", "G"), 14, anchor="start")

# ---- 每个分支的 6 个故障现象 + 底事件 ----
for b in BRANCHES:
    cx = b["x"]
    trunk_bottom = ROW_CYS[-1] - 40
    line(cx, G_CY + GATE_H / 2, cx, X_CY - X_R)
    for i, (eid, label) in enumerate(b["events"]):
        row, col = divmod(i, 2)          # 每行两个：左、右
        nx = cx - COL_OFF if col == 0 else cx + COL_OFF
        ny = ROW_CYS[row]
        approach = ny - 40
        if col == 0:
            line(cx, approach, nx, approach)
        else:
            line(cx, approach, nx, approach)
        line(nx, approach, nx, ny - NODE_H / 2)
        rect(nx, ny, NODE_W, NODE_H, "ev-ph")
        text(nx, ny, f"{eid} {label}", 14)

    # 底事件
    bx = cx
    add(f'<circle class="ev-basic" cx="{bx}" cy="{X_CY}" r="{X_R}"/>')
    text(bx, X_CY - 11, b["basic"][0], 17, weight="700")
    text(bx, X_CY + 13, b["basic"][1], 13)

# ---- 图例 ----
ly = 1000
or_gate(150, ly, 52, 42)
text(212, ly, "或门", 15, anchor="start", cls="legend")
rect(340, ly, 58, 36, "ev-mid")
text(414, ly, "中间事件", 15, anchor="start", cls="legend")
add(f'<circle class="ev-basic" cx="600" cy="{ly}" r="18"/>')
text(630, ly, "底事件", 15, anchor="start", cls="legend")

# ---- 图注 ----
text(950, 1026, "图 1  整流罩质量问题故障树（或门逻辑）", 16, cls="caption")

add("</svg>")

with open("/Users/q_kevin/projects/网关/报告写作/整流罩故障树/整流罩故障树.svg", "w", encoding="utf-8") as f:
    f.write("\n".join(out))
print("SVG written")
