import React, { useCallback, useEffect, useState, useMemo } from 'react';
import {
  Card,
  Table,
  Button,
  Tag,
  Breadcrumb,
  Empty,
  Skeleton,
  message,
  Space,
  Input,
  Tabs,
  Select,
  Flex,
} from 'antd';
import {
  HomeOutlined,
  EditOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import { Link, useNavigate, useParams, useSearchParams } from 'react-router-dom';
import agent from '@/api/agent';
import type { MissingGrade, Sheet } from '@/types';
import dayjs from 'dayjs';
import { useTranslation } from 'react-i18next';

const TEST_STATUS_PASSED = 1;

const GradingQueue: React.FC = () => {
  const { courseId } = useParams<{ courseId: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const [searchParams, setSearchParams] = useSearchParams();

  const tabParam = searchParams.get('tab');
  const activeTab: 'pending' | 'graded' = tabParam === 'graded' ? 'graded' : 'pending';
  const setActiveTab = (tab: 'pending' | 'graded') => {
    setSearchParams((prev) => {
      const next = new URLSearchParams(prev);
      next.set('tab', tab);
      return next;
    }, { replace: true });
  };
  const [missing, setMissing] = useState<MissingGrade[]>([]);
  const [graded, setGraded] = useState<MissingGrade[]>([]);
  const [sheets, setSheets] = useState<Sheet[]>([]);
  const [selectedSheetId, setSelectedSheetId] = useState<number>(0);
  const [selectedTaskId, setSelectedTaskId] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useState('');

  const cId = Number(courseId);

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [missingData, gradedData, sheetsData] = await Promise.all([
        agent.Courses.getMissingGrades(cId).catch(() => []),
        agent.Courses.getGradedSubmissions(cId).catch(() => []),
        agent.Sheets.getAll(cId).catch(() => []),
      ]);
      setMissing(missingData);
      setGraded(gradedData);
      setSheets(sheetsData);
    } catch {
      message.error(t('messages.updateFailed'));
    } finally {
      setLoading(false);
    }
  }, [cId, t]);

  useEffect(() => {
    if (courseId) fetchData();
  }, [courseId, fetchData]);

  const normalizedQuery = query.trim().toLowerCase();

  const filteredMissing = useMemo(() => {
    let list = missing;
    if (normalizedQuery) {
      list = list.filter((m) => {
        const user = m.grade.user;
        const name = user ? `${user.first_name} ${user.last_name} ${user.email}`.toLowerCase() : '';
        return name.includes(normalizedQuery);
      });
    }
    if (selectedSheetId !== 0) {
      list = list.filter((m) => m.sheet_id === selectedSheetId);
    }
    if (selectedTaskId !== null) {
      list = list.filter((m) => m.task_id === selectedTaskId);
    }
    return list;
  }, [missing, normalizedQuery, selectedSheetId, selectedTaskId]);

  const filteredGraded = useMemo(() => {
    let list = graded;
    if (normalizedQuery) {
      list = list.filter((m) => {
        const user = m.grade.user;
        const name = user ? `${user.first_name} ${user.last_name} ${user.email}`.toLowerCase() : '';
        return name.includes(normalizedQuery);
      });
    }
    if (selectedSheetId !== 0) {
      list = list.filter((m) => m.sheet_id === selectedSheetId);
    }
    if (selectedTaskId !== null) {
      list = list.filter((m) => m.task_id === selectedTaskId);
    }
    return list;
  }, [graded, normalizedQuery, selectedSheetId, selectedTaskId]);

  const commonColumns = [
    {
      title: t('grades.student'),
      key: 'student',
      render: (_: unknown, record: MissingGrade) =>
        record.grade.user ? (
          <Space direction="vertical" size={0}>
            <Space wrap>
              <span>
                {record.grade.user.first_name} {record.grade.user.last_name}
              </span>
            </Space>
            <span style={{ opacity: 0.65, fontSize: 12 }}>{record.grade.user.email}</span>
          </Space>
        ) : (
          `#${record.grade.submission_id}`
        ),
    },
    {
      title: t('grades.sheet'),
      dataIndex: 'sheet_id',
      key: 'sheet_id',
      render: (sheetId: number) => {
        const s = sheets.find((sh) => sh.id === sheetId);
        return s ? s.name : `${t('grades.sheet')} ${sheetId}`;
      },
      responsive: ['sm' as const],
    },
    {
      title: t('grades.task'),
      dataIndex: 'task_id',
      key: 'task_id',
      render: (taskId: number) => `${t('grades.task')} #${taskId}`,
      responsive: ['sm' as const],
    },
    {
      title: t('submission.publicTests'),
      key: 'tests',
      render: (_: unknown, record: MissingGrade) =>
        record.grade.public_test_status === TEST_STATUS_PASSED ? (
          <Tag icon={<CheckCircleOutlined />} color="success">
            {t('submission.testsPassed')}
          </Tag>
        ) : (
          <Tag icon={<CloseCircleOutlined />} color="error">
            {t('submission.testsFailed')}
          </Tag>
        ),
    },
  ];

  const pendingColumns = [
    ...commonColumns,
    {
      title: t('common.actions'),
      key: 'actions',
      render: (_: unknown, record: MissingGrade) => (
        <Button
          type="primary"
          icon={<EditOutlined />}
          onClick={() => navigate(`/courses/${cId}/grades/${record.grade.id}`)}
        >
          {t('grades.openGrade')}
        </Button>
      ),
    },
  ];

  const gradedColumns = [
    ...commonColumns,
    {
      title: t('grades.score'),
      key: 'score',
      render: (_: unknown, record: MissingGrade) => (
        <Tag color="blue">{record.grade.acquired_points} pts</Tag>
      ),
    },
    {
      title: t('grades.feedback'),
      key: 'feedback',
      render: (_: unknown, record: MissingGrade) => (
        <span style={{ maxWidth: 200, display: 'inline-block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
          {record.grade.feedback || '—'}
        </span>
      ),
    },
    {
      title: t('grades.gradedAt'),
      key: 'updated_at',
      render: (_: unknown, record: MissingGrade) =>
        dayjs(record.grade.updated_at).format('MMM D, HH:mm'),
    },
    {
      title: t('common.actions'),
      key: 'actions',
      render: (_: unknown, record: MissingGrade) => (
        <Button
          icon={<EditOutlined />}
          onClick={() => navigate(`/courses/${cId}/grades/${record.grade.id}`)}
        >
          {t('grades.amendGrade')}
        </Button>
      ),
    },
  ];

  return (
    <div>
      <Breadcrumb
        items={[
          { title: <Link to="/"><HomeOutlined /></Link> },
          { title: <Link to="/courses">{t('nav.courses')}</Link> },
          { title: <Link to={`/courses/${courseId}`}>{t('course.course')}</Link> },
          { title: t('grades.gradingQueue') },
        ]}
        style={{ marginBottom: 16 }}
      />

      <Card>
        <Flex justify="space-between" align="center" wrap="wrap" gap="middle" style={{ marginBottom: 16 }}>
          <Tabs
            activeKey={activeTab}
            onChange={(k) => setActiveTab(k as 'pending' | 'graded')}
            style={{ marginBottom: 0 }}
            items={[
              {
                key: 'pending',
                label: `${t('grades.needsReview')} (${missing.length})`,
              },
              {
                key: 'graded',
                label: `${t('grades.gradedSubmissions')} (${graded.length})`,
              },
            ]}
          />

          <Space wrap>
            <Select
              allowClear
              placeholder={t('grades.filterBySheet')}
              value={selectedSheetId}
              onChange={(val) => setSelectedSheetId(val ?? 0)}
              style={{ width: 170 }}
              options={[
                { value: 0, label: t('grades.allSheets') },
                ...sheets.map((s) => ({ value: s.id, label: s.name })),
              ]}
            />
            <Input
              allowClear
              placeholder={t('grades.taskId')}
              value={selectedTaskId ? String(selectedTaskId) : ''}
              onChange={(e) => {
                const val = e.target.value.trim();
                setSelectedTaskId(val ? Number(val) : null);
              }}
              style={{ width: 100 }}
            />
            <Input
              allowClear
              prefix={<SearchOutlined />}
              placeholder={t('enrollment.searchPlaceholder')}
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              style={{ width: 220 }}
              aria-label={t('common.search')}
            />
          </Space>
        </Flex>

        {loading ? (
          <Skeleton active paragraph={{ rows: 6 }} />
        ) : activeTab === 'pending' ? (
          filteredMissing.length === 0 ? (
            <Empty description={t('grades.noMissing')} />
          ) : (
            <Table
              dataSource={filteredMissing}
              columns={pendingColumns as any}
              rowKey={(record) => record.grade.id}
              pagination={{ pageSize: 15 }}
              scroll={{ x: 'max-content' }}
            />
          )
        ) : filteredGraded.length === 0 ? (
          <Empty description={t('grades.noGradedSubmissions')} />
        ) : (
          <Table
            dataSource={filteredGraded}
            columns={gradedColumns as any}
            rowKey={(record) => record.grade.id}
            pagination={{ pageSize: 15 }}
            scroll={{ x: 'max-content' }}
          />
        )}
      </Card>
    </div>
  );
};

export default GradingQueue;
