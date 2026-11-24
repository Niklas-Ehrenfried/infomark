import React, { useCallback, useEffect, useState } from 'react';
import {
  Card,
  Space,
  Button,
  Input,
  Checkbox,
  Alert,
  List,
  Avatar,
  Tag,
  message,
  Typography,
  Skeleton,
  Flex,
} from 'antd';
import {
  TeamOutlined,
  UserOutlined,
  CopyOutlined,
  KeyOutlined,
  UsergroupAddOutlined,
  CheckCircleOutlined,
  InfoCircleOutlined,
} from '@ant-design/icons';
import agent from '@/api/agent';
import type { Course, Team } from '@/types';
import dayjs from 'dayjs';
import { useTranslation } from 'react-i18next';

const { Paragraph, Text } = Typography;

interface TeamPanelProps {
  courseId: number;
  course: Course;
}

export const TeamPanel: React.FC<TeamPanelProps> = ({ courseId, course }) => {
  const { t } = useTranslation();
  const [team, setTeam] = useState<Team | null>(null);
  const [loading, setLoading] = useState(true);
  const [tokenLoading, setTokenLoading] = useState(false);
  const [joinLoading, setJoinLoading] = useState(false);

  const [inputToken, setInputToken] = useState('');
  const [consentChecked, setConsentChecked] = useState(false);

  const maxTeamSize = course.max_team_size || 2;
  const isFormationOpen = course.team_formation_open ?? false;

  const fetchTeam = useCallback(async () => {
    setLoading(true);
    try {
      const data = await agent.Teams.get(courseId);
      setTeam(data);
    } catch (err: any) {
      // 404 or not initialized yet
      setTeam(null);
    } finally {
      setLoading(false);
    }
  }, [courseId]);

  useEffect(() => {
    fetchTeam();
  }, [fetchTeam]);

  const handleGenerateToken = async () => {
    setTokenLoading(true);
    try {
      await agent.Teams.generateToken(courseId);
      message.success(t('team.tokenGenerated'));
      await fetchTeam();
    } catch (err: any) {
      message.error(err.message || t('team.tokenGenerateFailed'));
    } finally {
      setTokenLoading(false);
    }
  };

  const handleCopyToken = () => {
    if (team?.invite_token) {
      navigator.clipboard.writeText(team.invite_token);
      message.success(t('team.tokenCopied'));
    }
  };

  const handleJoinTeam = async () => {
    if (!inputToken.trim()) {
      message.error(t('team.tokenRequired'));
      return;
    }
    if (!consentChecked) {
      message.error(t('team.consentRequired'));
      return;
    }

    setJoinLoading(true);
    try {
      await agent.Teams.join(courseId, {
        token: inputToken.trim(),
        consent: consentChecked,
      });
      message.success(t('team.joinedSuccess'));
      setInputToken('');
      setConsentChecked(false);
      await fetchTeam();
    } catch (err: any) {
      message.error(err.message || t('team.joinFailed'));
    } finally {
      setJoinLoading(false);
    }
  };

  if (loading) {
    return <Skeleton active paragraph={{ rows: 6 }} />;
  }

  const membersCount = team?.members?.length || 1;
  const isTeamFull = membersCount >= maxTeamSize;
  const isSolo = membersCount === 1;

  return (
    <Space direction="vertical" size="large" style={{ width: '100%' }}>
      {!isFormationOpen && (
        <Alert
          type="warning"
          showIcon
          icon={<InfoCircleOutlined />}
          message={t('team.formationClosedTitle')}
          description={t('team.formationClosedDesc')}
        />
      )}

      {course.enforce_same_exercise_group && (
        <Alert
          type="info"
          showIcon
          message={t('team.enforceGroupNoticeTitle')}
          description={t('team.enforceGroupNoticeDesc')}
        />
      )}

      {/* Current Team Members Card */}
      <Card
        title={
          <Flex align="center" gap="small">
            <TeamOutlined />
            <span>{t('team.myTeam')} (Team {team?.team_number || team?.id})</span>
            <Tag color={isTeamFull ? 'orange' : 'blue'}>
              {membersCount} / {maxTeamSize} {t('team.membersCount')}
            </Tag>
          </Flex>
        }
      >
        <List
          itemLayout="horizontal"
          dataSource={team?.members || []}
          renderItem={(member) => (
            <List.Item>
              <List.Item.Meta
                avatar={
                  member.avatar_url ? (
                    <Avatar src={member.avatar_url} />
                  ) : (
                    <Avatar icon={<UserOutlined />} />
                  )
                }
                title={`${member.first_name} ${member.last_name}`}
                description={`${t('team.joinedAt')}: ${dayjs(member.joined_at).format('MMM D, YYYY HH:mm')}`}
              />
            </List.Item>
          )}
        />
      </Card>

      {/* Invite Teammate Card */}
      {!isTeamFull && isFormationOpen && (
        <Card
          title={
            <Flex align="center" gap="small">
              <KeyOutlined />
              <span>{t('team.invitePartner')}</span>
            </Flex>
          }
        >
          <Paragraph type="secondary">{t('team.inviteInstructions')}</Paragraph>

          {team?.invite_token ? (
            <Space direction="vertical" style={{ width: '100%' }}>
              <Input.Search
                readOnly
                value={team.invite_token}
                enterButton={
                  <Button icon={<CopyOutlined />}>{t('team.copyToken')}</Button>
                }
                onSearch={handleCopyToken}
              />
              <Flex justify="flex-end" gap="small">
                <Button
                  loading={tokenLoading}
                  onClick={handleGenerateToken}
                  size="small"
                >
                  {t('team.refreshToken')}
                </Button>
              </Flex>
            </Space>
          ) : (
            <Button
              type="primary"
              icon={<KeyOutlined />}
              loading={tokenLoading}
              onClick={handleGenerateToken}
            >
              {t('team.generateToken')}
            </Button>
          )}
        </Card>
      )}

      {/* Join Another Team Card (only possible while in solo team and formation is open) */}
      {isSolo && isFormationOpen && (
        <Card
          title={
            <Flex align="center" gap="small">
              <UsergroupAddOutlined />
              <span>{t('team.joinAnotherTeam')}</span>
            </Flex>
          }
        >
          <Paragraph type="secondary">{t('team.joinInstructions')}</Paragraph>
          <Space direction="vertical" size="middle" style={{ width: '100%' }}>
            <Input
              placeholder={t('team.tokenPlaceholder')}
              value={inputToken}
              onChange={(e) => setInputToken(e.target.value)}
              prefix={<KeyOutlined />}
            />

            <Checkbox
              checked={consentChecked}
              onChange={(e) => setConsentChecked(e.target.checked)}
            >
              <Text strong>{t('team.gdprConsentText')}</Text>
            </Checkbox>

            <Button
              type="primary"
              icon={<CheckCircleOutlined />}
              loading={joinLoading}
              disabled={!inputToken.trim() || !consentChecked}
              onClick={handleJoinTeam}
            >
              {t('team.joinButton')}
            </Button>
          </Space>
        </Card>
      )}

      {!isSolo && (
        <Alert
          type="info"
          showIcon
          message={t('team.lockInNoticeTitle')}
          description={t('team.lockInNoticeDesc')}
        />
      )}
    </Space>
  );
};

export default TeamPanel;
