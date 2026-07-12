import React from "react";

interface PanelProps {
  title: string;
  rightTag?: string;
  className?: string;
  children: React.ReactNode;
}

export function Panel({ title, rightTag, className = "", children }: PanelProps) {
  return (
    <div className={`flex flex-col border border-gray-200 bg-white rounded-lg overflow-hidden shadow-sm ${className}`}>
      <div className="flex justify-between items-center px-3 py-1.5 bg-gray-100 text-xs uppercase tracking-wider border-b border-gray-200">
        <span className="font-bold text-gray-700">{title}</span>
        {rightTag && <span className="text-gray-500">{rightTag}</span>}
      </div>
      <div className="flex-1 min-h-0 flex flex-col overflow-hidden">{children}</div>
    </div>
  );
}
