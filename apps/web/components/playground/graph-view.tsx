"use client";

import * as React from "react";
import {
  Background,
  BackgroundVariant,
  Controls,
  Handle,
  MarkerType,
  Position,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
  type Edge,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Bot, GitBranch, Plug, Send, Wrench, Zap, type LucideIcon } from "lucide-react";
import { cn } from "@/lib/utils";
import { kindOf, type PublicNode } from "@/lib/playground";

const NODE_W = 208;
const NODE_H = 58;
const COL_W = NODE_W + 44;
const ROW_H = 92;
const PAD = 28;
/** Below this zoom node text is unreadable, so wide graphs pan instead of shrinking. */
const MIN_FIT_ZOOM = 0.72;

const KIND: Record<ReturnType<typeof kindOf>, { icon: LucideIcon; tint: string }> = {
  trigger: { icon: Zap, tint: "bg-warn-bg text-warn" },
  agent: { icon: Bot, tint: "bg-accent-bg text-accent-soft" },
  tool: { icon: Wrench, tint: "bg-running-bg text-running" },
  logic: { icon: GitBranch, tint: "bg-neutral-bg text-muted" },
  output: { icon: Send, tint: "bg-success-bg text-success" },
  app: { icon: Plug, tint: "bg-running-bg text-running" },
};

type GraphNode = Node<{ node: PublicNode; selected: boolean }, "compiled">;

function CompiledNode({ data }: NodeProps<GraphNode>) {
  const { node, selected } = data;
  const kind = KIND[kindOf(node)];
  const Icon = kind.icon;
  return (
    <div
      className={cn(
        "playground-node w-[208px] rounded-lg border bg-surface px-3 py-2.5 shadow-card transition-colors",
        selected ? "border-accent ring-2 ring-accent/20" : "border-border-strong hover:border-accent-soft",
      )}
      style={{ animationDelay: `${Math.min(node.col, 12) * 70}ms` }}
    >
      <Handle type="target" position={Position.Left} className="!size-2 !border-0 !bg-border-strong" isConnectable={false} />
      <div className="flex items-center gap-2.5">
        <span className={cn("flex size-7 shrink-0 items-center justify-center rounded-md", kind.tint)}>
          <Icon className="size-3.5" />
        </span>
        <div className="min-w-0">
          <div className="truncate text-[12.5px] font-medium leading-tight text-fg">{node.name || node.key}</div>
          <div className="mt-0.5 truncate font-mono text-[10.5px] text-faint">
            {node.service ? `${node.service} · ${node.label ?? ""}` : node.type}
          </div>
        </div>
      </div>
      <Handle type="source" position={Position.Right} className="!size-2 !border-0 !bg-border-strong" isConnectable={false} />
    </div>
  );
}

const nodeTypes = { compiled: CompiledNode };

function Canvas({
  nodes,
  selected,
  onSelect,
}: {
  nodes: PublicNode[];
  selected: string | null;
  onSelect: (key: string | null) => void;
}) {
  const flow = useReactFlow();
  const wrapper = React.useRef<HTMLDivElement>(null);

  const flowNodes = React.useMemo<GraphNode[]>(
    () =>
      nodes.map((node) => ({
        id: node.key,
        type: "compiled",
        position: { x: node.col * COL_W, y: node.row * ROW_H },
        data: { node, selected: node.key === selected },
        draggable: false,
      })),
    [nodes, selected],
  );

  const edges = React.useMemo<Edge[]>(
    () =>
      nodes.flatMap((node) =>
        (node.dependsOn ?? []).map((dep) => {
          const touching = selected === node.key || selected === dep;
          return {
            id: `${dep}->${node.key}`,
            source: dep,
            target: node.key,
            type: "smoothstep",
            style: touching ? { stroke: "var(--color-accent-soft)", strokeWidth: 2 } : undefined,
            markerEnd: { type: MarkerType.ArrowClosed, width: 16, height: 16, color: "var(--color-faint)" },
          };
        }),
      ),
    [nodes, selected],
  );

  // Fit the graph when it's readable. When it's too wide, start at the trigger.
  React.useEffect(() => {
    const el = wrapper.current;
    if (!el || nodes.length === 0) return;
    const place = () => {
      const graphW = Math.max(...nodes.map((n) => n.col)) * COL_W + NODE_W;
      const graphH = Math.max(...nodes.map((n) => n.row)) * ROW_H + NODE_H;
      const { width, height } = el.getBoundingClientRect();
      const fit = Math.min((width - PAD * 2) / graphW, (height - PAD * 2) / graphH, 1);
      const zoom = Math.max(fit, MIN_FIT_ZOOM);
      const center = (space: number, size: number) => Math.max(PAD, (space - size * zoom) / 2);
      void flow.setViewport({ x: center(width, graphW), y: center(height, graphH), zoom });
    };
    const t = setTimeout(place, 40);
    return () => clearTimeout(t);
  }, [nodes, flow]);

  return (
    <div ref={wrapper} className="absolute inset-0">
    <ReactFlow
      nodes={flowNodes}
      edges={edges}
      nodeTypes={nodeTypes}
      onNodeClick={(_, n) => onSelect(n.id)}
      onPaneClick={() => onSelect(null)}
      nodesConnectable={false}
      elementsSelectable={false}
      minZoom={0.2}
      proOptions={{ hideAttribution: true }}
    >
      <Background variant={BackgroundVariant.Dots} gap={22} size={1} color="var(--color-border-strong)" />
      <Controls showInteractive={false} />
    </ReactFlow>
    </div>
  );
}

export function GraphView(props: {
  nodes: PublicNode[];
  selected: string | null;
  onSelect: (key: string | null) => void;
}) {
  return (
    <ReactFlowProvider>
      <Canvas {...props} />
    </ReactFlowProvider>
  );
}
