#!/usr/bin/python
# -*- coding: UTF-8 -*-
 
class ExpGolombObject(object):
    def __init__(self, value, bit_length=0):
        self._bit_count = value.bit_length()
        self._bit_length = bit_length or self._bit_count
        self._zero_count = self._bit_length - self._bit_count
        self.value = value
        self.value_encoded = 0
 
        if self._bit_length < self._bit_count:
            raise Exception("bit length of value {} should be {} at least", self.value, self._bit_count)
 
    def __bool__(self):
        return 0 != self.value
 
    @property
    def zero_count(self):
        return self._zero_count
 
    @property
    def bit_count(self):
        return self._bit_count
 
    def encode(self):
        self._bit_count = self.value.bit_length()
        self._zero_count = self._bit_length - self._bit_count
        self.value_encoded = self.value
 
 
class FixedPatternBin(ExpGolombObject):
    def __init__(self, value=0):
        super(FixedPatternBin, self).__init__(value, 1)
 
 
class FixedLenUint(ExpGolombObject):
    def __init__(self, bit_length, value=0):
        super(FixedLenUint, self).__init__(value, bit_length)
 
    def set_data(self, datas, bit_index):
        byte_index = divmod(bit_index, 8)[0]
        bit = bit_index - byte_index * 8
 
        self.value = (datas[byte_index] << 24) + (datas[byte_index + 1] << 16) + (datas[byte_index + 2] << 8) + (datas[byte_index + 3])
 
        if bit + self._bit_count <= 32:
            self.value = (self.value << bit) & 0xffffffff
            self.value = (self.value >> (32 - self._bit_count)) & 0xffffffff
 
        else:
            tmp = self._bit_count + bit - 32
 
            self.value = (self.value << bit) & 0xffffffff
            self.value = (self.value >> bit)
            self.value = (self.value << tmp) & 0xffffffff
 
            # the fifth byte
            self.value += (datas[byte_index + 4] >> (8 - tmp))
 
 
class ExpGolombNumber(ExpGolombObject):
    def __init__(self, value):
        super(ExpGolombNumber, self).__init__(value)
 
    def __str__(self):
        return "zero_count: {}, bit_count:{}, value: {}".format(self.zero_count, self.bit_count, self.value)
 
    def set_data(self, datas, start_bit_index):
        """
        :param datas:
        :param start_bit_index:  比特索引值，0开始计数
        :return:
        """
        self.value = 0
        self._zero_count = 0
        self._bit_count = 0
 
        byte_index = divmod(start_bit_index, 8)[0]
        byte = datas[byte_index]
        bit_index = start_bit_index - byte_index * 8
 
        if ((byte << bit_index) & 0xff) <= 0x80:
            bit_index = start_bit_index - byte_index * 8
 
            while True:
                if ((byte << bit_index) & 0xff) >= 0x80:
                    break
 
                self._zero_count += 1
                start_bit_index += 1
                bit_index += 1
 
                if 8 == bit_index:
                    byte_index = divmod(start_bit_index, 8)[0]
                    byte = datas[byte_index]
                    bit_index = 0
 
        self._bit_count = self._zero_count + 1
 
        if self._bit_count > 8:
            return False
 
        if bit_index + self._bit_count <= 8:
            mask = 0xff >> bit_index
            self.value_encoded = (datas[byte_index] & mask) >> (8 - bit_index - self._bit_count)
 
        else:
            mask = 0xff >> bit_index
            self.value_encoded = datas[byte_index] & mask
            self.value_encoded = (self.value_encoded << (self._bit_count - (8 - bit_index))) & 0xff
 
            byte_index += 1
            self.value_encoded += (datas[byte_index] >> (16 - bit_index - self._bit_count))
 
        return True
 
 
class ExpGolombUE(ExpGolombNumber):
    def __init__(self, value=0):
        super(ExpGolombUE, self).__init__(value)
 
    def encode(self):
        self.value_encoded = self.value + 1
        self._bit_count = self.value_encoded.bit_length()
        self._zero_count = self._bit_count - 1
 
    def set_data(self, datas, start_bit_index):
        if super(ExpGolombUE, self).set_data(datas, start_bit_index):
            self.value = self.value_encoded - 1
 
 
class ExpGolombSE(ExpGolombNumber):
    def __init__(self, value=0):
        super(ExpGolombSE, self).__init__(value)
 
    def encode(self):
        tmp = abs(self.value)
 
        if 0 == self.value:
            self._bit_count = 1
            self._zero_count = 0
            self.value_encoded = 1
            return
 
        self._bit_count = tmp.bit_length()
        self._zero_count = self._bit_count
 
        tmp = tmp << 1
        self._bit_count += 1
 
        if self.value < 0:
            tmp += 1
 
        self.value_encoded = tmp
 
    def set_data(self, datas, start_bit_index):
        if super(ExpGolombSE, self).set_data(datas, start_bit_index):
            sign = self.value_encoded & 0x01
            self.value = self.value_encoded >> 1
 
            if sign:
                self.value *= -1
 
 
class ExpGolombCodec:
    def __init__(self, buffer):
        self.__value = 0
        self.__prefix = 0
        self.__buffer = buffer
 
    def __str__(self):
        return "prefix={}, value={}, data length={}".format(self.__prefix, bin(self.__value), len(self.__buffer))
 
    def __add_coded(self, value):
        if len(self.__buffer) > 3 and value <= 0x03:
            if not self.__buffer[-2] and not self.__buffer[-1]:
                self.__buffer.append(0x03)
        self.__buffer.append(value)
 
    def __add_item(self, glbobj):
        self.__value |= (glbobj.value_encoded << 40 - self.__prefix - glbobj.zero_count - glbobj.bit_count)
        self.__prefix += glbobj.zero_count + glbobj.bit_count
 
    def __check_prefix(self, glbobj):
        if self.__prefix + glbobj.zero_count + glbobj.bit_count > 40:
            if self.__prefix >= 32:
                tmp = self.__value & 0xffffffff00
                tmp = tmp >> 8
 
                self.__add_coded(tmp >> 24 & 0xff)
                self.__add_coded(tmp >> 16 & 0xff)
                self.__add_coded(tmp >> 8 & 0xff)
                self.__add_coded(tmp & 0xff)
 
                self.__value = (self.__value << 32) & 0xff00000000
                self.__prefix -= 32
 
            elif self.__prefix >= 24:
                tmp = self.__value & 0xffffff0000
                tmp = tmp >> 16
 
                self.__add_coded(tmp >> 16)
                self.__add_coded(tmp >> 8 & 0xff)
                self.__add_coded(tmp & 0xff)
 
                self.__value = (self.__value << 24) & 0xff00000000
                self.__prefix -= 24
 
            elif self.__prefix >= 16:
                tmp = self.__value & 0xffff000000
                tmp = tmp >> 24
 
                self.__add_coded(tmp >> 8 & 0xff)
                self.__add_coded(tmp & 0xff)
 
                self.__value = (self.__value << 16) & 0xff00000000
                self.__prefix -= 16
 
            elif self.__prefix >= 8:
                tmp = self.__value & 0xff00000000
                tmp = tmp >> 32
 
                self.__add_coded(tmp & 0xff)
 
                self.__value = (self.__value << 8) & 0xff00000000
                self.__prefix -= 8
 
            if 8 == self.__prefix:
                self.__add_coded(self.__value >> 32 & 0xff)
                self.__value = 0
                self.__prefix = 0
 
    def byte_aligned(self):
        return 0 == self.__prefix % 8
 
    def add_trail(self):
        ret = self.__prefix % 8
 
        if ret:
            self.__buffer[-1] |= (1 << (8 - ret - 1))
        else:
            self.__add_coded(0x80)
 
    def reset(self):
        self.__value = 0
        self.__prefix = 0
        self.__buffer.clear()
 
    def encode_done(self, trail_bit):
        if self.__prefix > 32:
            self.__add_coded(self.__value >> 32 & 0xff)
            self.__add_coded(self.__value >> 24 & 0xff)
            self.__add_coded(self.__value >> 16 & 0xff)
            self.__add_coded(self.__value >> 8 & 0xff)
            self.__add_coded(self.__value & 0xff)
 
        elif self.__prefix > 24:
            self.__add_coded(self.__value >> 32 & 0xff)
            self.__add_coded(self.__value >> 24 & 0xff)
            self.__add_coded(self.__value >> 16 & 0xff)
            self.__add_coded(self.__value >> 8 & 0xff)
 
        elif self.__prefix > 16:
            self.__add_coded(self.__value >> 32 & 0xff)
            self.__add_coded(self.__value >> 24 & 0xff)
            self.__add_coded(self.__value >> 16 & 0xff)
 
        elif self.__prefix > 8:
            self.__add_coded(self.__value >> 32 & 0xff)
            self.__add_coded(self.__value >> 24 & 0xff)
 
        else:
            self.__add_coded(self.__value >> 32 & 0xff)
 
        if trail_bit:
            self.add_trail()
 
        print(str(self))
        self.__prefix = 0
        self.__value = 0
 
    def append(self, glbobj, dbg=False):
        glbobj.encode()
        self.__check_prefix(glbobj)
 
        if dbg:
            print("before append {} : {}".format(glbobj.value_encoded, self))
 
        self.__add_item(glbobj)
 
        if dbg:
            print("after append {} : {}".format(glbobj.value_encoded, self))
 
    def encode_ue(self, value_list):
        self.__value = 0
        self.__prefix = 0
        self.__buffer = []
 
        for x in value_list:
            y = x
 
            if type(x) is not int:
                y = ord(x)
 
            if y > 0xff:
                raise Exception("must be byte: {}".format(y))
 
            e = ExpGolombUE(y)
            self.append(e)