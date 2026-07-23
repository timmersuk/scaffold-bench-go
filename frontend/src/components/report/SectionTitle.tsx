export function SectionTitle({ children }: { children: string }) {
  return (
    <h2 className="text-xs uppercase tracking-widest text-gray-500 border-b border-gray-200 pb-2 mb-3">
      {children}
    </h2>
  );
}
