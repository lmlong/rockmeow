import { TaskBoard } from "@/components/board/TaskBoard";

export default function Home() {
  return (
    <main className="min-h-screen p-6 bg-gray-50">
      <div className="max-w-7xl mx-auto">
        <TaskBoard />
      </div>
    </main>
  );
}
