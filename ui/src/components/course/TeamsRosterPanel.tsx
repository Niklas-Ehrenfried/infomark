import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  Table,
  Input,
  Button,
  Space,
  Tag,
  Modal,
  Select,
  Popconfirm,
  Alert,
  Skeleton,
  message,
  Flex,
  Typography,
} from 'antd';

const { Text } = Typography;
import {
  SearchOutlined,
  SwapOutlined,
  ScissorOutlined,
  PlusOutlined,
  ExclamationCircleOutlined,
} from '@ant-design/icons';
import agent from '@/api/agent';
import type { Course, Group, TeamRosterMember } from '@/types';
import { useTranslation } from 'react-i18next';

interface TeamsRosterPanelProps {
  courseId: number;
  course: Course;
  canManage: boolean;
  isAdmin?: boolean;
}

export const TeamsRosterPanel: React.FC<TeamsRosterPanelProps> = ({
  courseId,
  course,
  canManage,
  isAdmin = false,
}) => {
  const { t } = useTranslation();
  const [roster, setRoster] = useState<TeamRosterMember[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');
  const [groups, setGroups] = useState<Group[]>([]);
  const [selectedGroupId, setSelectedGroupId] = useState<number>(0);

  // Move Modal State
  const [moveModalVisible, setMoveModalVisible] = useState(false);
  const [selectedStudent, setSelectedStudent] = useState<TeamRosterMember | null>(null);
  const [targetTeamId, setTargetTeamId] = useState<number | null>(null);
  const [moveLoading, setMoveLoading] = useState(false);

  // Custom Team Modal State
  const [customTeamModalVisible, setCustomTeamModalVisible] = useState(false);
  const [selectedUserIds, setSelectedUserIds] = useState<number[]>([]);
  const [customTeamLoading, setCustomTeamLoading] = useState(false);

  const maxTeamSize = course.max_team_size || 2;

  useEffect(() => {
    if (isAdmin) {
      agent.Groups.getAll(courseId).then(setGroups).catch(() => {});
    }
  }, [courseId, isAdmin]);

  const fetchRoster = useCallback(async () => {
    setLoading(true);
    try {
      const data = await agent.Teams.getRoster(courseId, { q: searchQuery });
      setRoster(data);
    } catch (err: any) {
      message.error(err.message || t('messages.fetchFailed'));
    } finally {
      setLoading(false);
    }
  }, [courseId, searchQuery, t]);

  useEffect(() => {
    fetchRoster();
  }, [fetchRoster]);

  const handleSplit = async (userId: number) => {
    try {
      await agent.Teams.splitMember(courseId, { user_id: userId });
      message.success(t('team.splitSuccess'));
      await fetchRoster();
    } catch (err: any) {
      message.error(err.message || t('team.splitFailed'));
    }
  };

  const handleMove = async () => {
    if (!selectedStudent || !targetTeamId) {
      return;
    }
    setMoveLoading(true);
    try {
      await agent.Teams.moveMember(courseId, {
        user_id: selectedStudent.user_id,
        target_team_id: targetTeamId,
      });
      message.success(t('team.moveSuccess'));
      setMoveModalVisible(false);
      setSelectedStudent(null);
      setTargetTeamId(null);
      await fetchRoster();
    } catch (err: any) {
      message.error(err.message || t('team.moveFailed'));
    } finally {
      setMoveLoading(false);
    }
  };

  const handleCreateCustomTeam = async () => {
    if (selectedUserIds.length < 1) {
      message.error(t('team.selectStudentsPrompt'));
      return;
    }
    setCustomTeamLoading(true);
    try {
      await agent.Teams.createCustomTeam(courseId, {
        user_ids: selectedUserIds,
      });
      message.success(t('team.customTeamSuccess'));
      setCustomTeamModalVisible(false);
      setSelectedUserIds([]);
      await fetchRoster();
    } catch (err: any) {
      message.error(err.message || t('team.customTeamFailed'));
    } finally {
      setCustomTeamLoading(false);
    }
  };

  // Identify unique existing teams for target selection
  const uniqueTeams = useMemo(() => {
    const map = new Map<number, { names: string[]; teamNumber?: number }>();
    for (const member of roster) {
      if (member.team_id) {
        const info = map.get(member.team_id) || { names: [], teamNumber: member.team_number };
        info.names.push(`${member.first_name} ${member.last_name}`);
        map.set(member.team_id, info);
      }
    }
    return Array.from(map.entries()).map(([teamId, info]) => ({
      teamId,
      label: `${t('team.teamNumber', { num: info.teamNumber ?? `#${teamId}` })} (${info.names.join(', ')})`,
      size: info.names.length,
    }));
  }, [roster, t]);

  const groupFilterOptions = useMemo(() => {
    if (isAdmin && groups.length > 0) {
      return groups.map((g) => ({
        value: g.id,
        label: g.description ? g.description : `${t('group.group')} #${g.id}`,
      }));
    }
    const seen = new Map<number, string>();
    for (const m of roster) {
      if (m.group_id && !seen.has(m.group_id)) {
        seen.set(m.group_id, m.group_description || `${t('group.group')} #${m.group_id}`);
      }
    }
    return Array.from(seen.entries()).map(([id, label]) => ({ value: id, label }));
  }, [isAdmin, groups, roster, t]);

  const filteredRoster = useMemo(() => {
    if (selectedGroupId === 0) return roster;
    return roster.filter((m) => m.group_id === selectedGroupId);
  }, [roster, selectedGroupId]);

  const hasOversizedTeams = useMemo(() => {
    return filteredRoster.some((m) => m.team_size > maxTeamSize);
  }, [filteredRoster, maxTeamSize]);

  const columns = [
    {
      title: t('fields.name', 'Name'),
      key: 'name',
      render: (_: unknown, record: TeamRosterMember) =>
        `${record.first_name} ${record.last_name}`,
    },
    {
      title: t('fields.studentNumber', 'Student Number'),
      dataIndex: 'student_number',
      key: 'student_number',
    },
    {
      title: t('fields.email', 'Email'),
      dataIndex: 'email',
      key: 'email',
    },
    {
      title: t('course.groups', 'Groups'),
      key: 'group_description',
      render: (_: unknown, record: TeamRosterMember) => {
        const desc = record.group_description || (record.group_id ? `Group #${record.group_id}` : null);
        return desc ? <Tag color="blue">{desc}</Tag> : <Text type="secondary">—</Text>;
      },
    },
    {
      title: t('team.teamInfo', 'Team Info'),
      key: 'team_info',
      render: (_: unknown, record: TeamRosterMember) => {
        if (!record.team_id) {
          return <Tag color="default">{t('team.noTeam', 'No Team')}</Tag>;
        }
        const isOversized = record.team_size > maxTeamSize;
        const teamLabel = t('team.teamNumber', { num: record.team_number ?? `#${record.team_id}` });
        return (
          <Space>
            <span>{teamLabel}</span>
            <Tag color={isOversized ? 'error' : record.team_size === 1 ? 'default' : 'cyan'}>
              {record.team_size} {record.team_size === 1 ? t('team.memberSingular') : t('team.membersCount')}
            </Tag>
          </Space>
        );
      },
    },
    ...(canManage
      ? [
          {
            title: t('common.actions'),
            key: 'actions',
            render: (_: unknown, record: TeamRosterMember) => (
              <Space size="small">
                <Button
                  size="small"
                  icon={<SwapOutlined />}
                  onClick={() => {
                    setSelectedStudent(record);
                    setTargetTeamId(null);
                    setMoveModalVisible(true);
                  }}
                >
                  {t('team.move')}
                </Button>
                {record.team_size > 1 && (
                  <Popconfirm
                    title={t('team.splitConfirmTitle')}
                    description={t('team.splitConfirmDesc')}
                    onConfirm={() => handleSplit(record.user_id)}
                    okText={t('common.yes')}
                    cancelText={t('common.no')}
                  >
                    <Button size="small" danger icon={<ScissorOutlined />}>
                      {t('team.split')}
                    </Button>
                  </Popconfirm>
                )}
              </Space>
            ),
          },
        ]
      : []),
  ];

  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      {hasOversizedTeams && (
        <Alert
          type="warning"
          showIcon
          icon={<ExclamationCircleOutlined />}
          message={t('team.oversizedWarningTitle')}
          description={t('team.oversizedWarningDesc', { max: maxTeamSize })}
        />
      )}

      <Flex justify="space-between" align="center" wrap="wrap" gap="small">
        <Space wrap>
          <Input.Search
            placeholder={t('team.searchPlaceholder')}
            allowClear
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            onSearch={fetchRoster}
            style={{ width: 260 }}
            prefix={<SearchOutlined />}
          />
          <Select
            allowClear
            placeholder={t('team.filterByGroup')}
            value={selectedGroupId}
            onChange={(val) => setSelectedGroupId(val ?? 0)}
            style={{ width: 220 }}
            options={[
              { value: 0, label: t('team.allGroups') },
              ...groupFilterOptions,
            ]}
          />
        </Space>

        {canManage && (
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => {
              setSelectedUserIds([]);
              setCustomTeamModalVisible(true);
            }}
          >
            {t('team.createCustomTeam')}
          </Button>
        )}
      </Flex>

      {loading ? (
        <Skeleton active paragraph={{ rows: 6 }} />
      ) : (
        <Table
          dataSource={filteredRoster}
          rowKey={(r) => r.user_id}
          columns={columns as any}
          pagination={{ pageSize: 20 }}
          scroll={{ x: 'max-content' }}
        />
      )}

      {/* Move Member Modal */}
      <Modal
        title={t('team.moveMemberTitle')}
        open={moveModalVisible}
        onOk={handleMove}
        onCancel={() => {
          setMoveModalVisible(false);
          setSelectedStudent(null);
          setTargetTeamId(null);
        }}
        confirmLoading={moveLoading}
        okButtonProps={{ disabled: !targetTeamId }}
      >
        {selectedStudent && (
          <Space direction="vertical" style={{ width: '100%' }}>
            <p>
              {t('team.moveStudentPrompt', {
                name: `${selectedStudent.first_name} ${selectedStudent.last_name}`,
              })}
            </p>
            <Select
              style={{ width: '100%' }}
              placeholder={t('team.selectTargetTeam')}
              value={targetTeamId}
              onChange={(val) => setTargetTeamId(val)}
              options={uniqueTeams
                .filter((team) => team.teamId !== selectedStudent.team_id)
                .map((team) => ({
                  value: team.teamId,
                  label: `${team.label}${team.size >= maxTeamSize ? ` (${t('team.fullWarning')})` : ''}`,
                }))}
            />
          </Space>
        )}
      </Modal>

      {/* Create Custom Team Modal */}
      <Modal
        title={t('team.createCustomTeamTitle')}
        open={customTeamModalVisible}
        onOk={handleCreateCustomTeam}
        onCancel={() => {
          setCustomTeamModalVisible(false);
          setSelectedUserIds([]);
        }}
        confirmLoading={customTeamLoading}
        okButtonProps={{ disabled: selectedUserIds.length < 1 }}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <p>{t('team.customTeamPrompt')}</p>
          <Select
            mode="multiple"
            style={{ width: '100%' }}
            placeholder={t('team.selectStudentsPlaceholder')}
            value={selectedUserIds}
            onChange={(vals) => setSelectedUserIds(vals)}
            options={roster.map((r) => ({
              value: r.user_id,
              label: `${r.first_name} ${r.last_name} (${r.student_number || r.email})`,
            }))}
            filterOption={(input, option) =>
              (option?.label ?? '').toLowerCase().includes(input.toLowerCase())
            }
          />
          {selectedUserIds.length > maxTeamSize && (
            <Alert
              type="warning"
              showIcon
              message={t('team.customTeamOversizeWarning', { max: maxTeamSize })}
            />
          )}
        </Space>
      </Modal>
    </Space>
  );
};

export default TeamsRosterPanel;
